// glel is an expression language for Go.
// glel is written in Lua programing language.
package glel

import (
	"context"
	"regexp"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
	luar "layeh.com/gopher-luar"
)

// DefaultAllowedFunctions is a list of allowed builtin functions.
const DefaultAllowedFunctions = `
  _VERSION assert error    ipairs   next pairs
  pcall    select tonumber tostring type unpack xpcall
  coroutine.create coroutine.resume coroutine.running coroutine.status
  coroutine.wrap   coroutine.yield
  math.abs   math.acos math.asin  math.atan math.atan2 math.ceil
  math.cos   math.cosh math.deg   math.exp  math.fmod  math.floor
  math.frexp math.huge math.ldexp math.log  math.log10 math.max
  math.min   math.modf math.pi    math.pow  math.rad   math.random
  math.sin   math.sinh math.sqrt  math.tan  math.tanh
  os.clock os.difftime os.time
  string.byte string.char  string.find  string.format string.gmatch
  string.gsub string.len   string.lower string.match  string.reverse
  string.sub  string.upper
  table.insert table.maxn table.remove table.sort 
`

// based on https://github.com/APItools/sandbox.lua/blob/master/sandbox.lua
const sandboxScript = `
local BASE_ENV = {}

local function allow_fn(id)
  local module, method = id:match('([^%.]+)%.([^%.]+)')
  if module then
    BASE_ENV[module]         = BASE_ENV[module] or {}
    BASE_ENV[module][method] = _G[module][method]
  else
    BASE_ENV[id] = _G[id]
  end
end

local allowed_fn = [[
  $$$
]]
allowed_fn:gsub('%S+', allow_fn)

local function protect_module(module, module_name)
  return setmetatable({}, {
    __index = module,
    __newindex = function(_, attr_name, _)
      error('Can not modify ' .. module_name .. '.' .. attr_name .. '. Protected by the sandbox.')
    end
  })
end

('coroutine math os string table'):gsub('%S+', function(module_name)
  BASE_ENV[module_name] = protect_module(BASE_ENV[module_name], module_name)
end)

if __envfunc then
  __envfunc(BASE_ENV)
end

function sandbox_call(f, nenv)
  local env = setmetatable(nenv or {}, {__index = BASE_ENV})
  env._G = env._G or env
  setfenv(f, env)
  local ok, result = pcall(f)
  if not ok then
    error(result) 
  end
  return result
end
`

type lStatePool interface {
	Get() *lua.LState
	Put(*lua.LState)
	Shutdown()
}

type nocacheLStatePool struct {
	lstate *lua.LState
}

func newNocacheLStatePool(factory func() *lua.LState) lStatePool {
	return &nocacheLStatePool{
		lstate: factory(),
	}
}

func (pl *nocacheLStatePool) Get() *lua.LState {
	return pl.lstate
}

func (pl *nocacheLStatePool) Put(_ *lua.LState) {
}

func (pl *nocacheLStatePool) Shutdown() {
	pl.lstate.Close()
}

type syncLStatePool struct {
	m       sync.Mutex
	factory func() *lua.LState
	pool    []*lua.LState
	limit   chan struct{}
}

func newSyncLStatePool(size int, factory func() *lua.LState) lStatePool {
	return &syncLStatePool{
		m:       sync.Mutex{},
		factory: factory,
		pool:    make([]*lua.LState, 0, size),
		limit:   make(chan struct{}, size),
	}
}

func (pl *syncLStatePool) Get() *lua.LState {
	pl.limit <- struct{}{}
	pl.m.Lock()
	defer pl.m.Unlock()
	n := len(pl.pool)
	if n == 0 {
		return pl.factory()
	}
	x := pl.pool[n-1]
	pl.pool = pl.pool[0 : n-1]
	return x
}

func (pl *syncLStatePool) Put(lstate *lua.LState) {
	pl.m.Lock()
	defer pl.m.Unlock()
	pl.pool = append(pl.pool, lstate)
	<-pl.limit
}

func (pl *syncLStatePool) Shutdown() {
	for _, L := range pl.pool {
		L.Close()
	}
}

// ExprConfig is a configurations for [Expr].
type ExprConfig struct {
	// DisableSandbox disables sandboxing.
	DisableSandbox bool

	// PoolSize is a size of the Lua VM pool.
	// This defaults to 50.
	// Pooling is disabled if size is a nevative value.
	PoolSize int

	// AllowedFunctions is a space separated list of allowed Lua buitin functions.
	// This defaults to [DefaultAllowedFunctions].
	AllowedFunctions string

	// EnvFunc is a function that customizes Lua environment.
	// Stack top of given *lua.LState is a table that will be used
	// in [Evaler].Eval.
	//
	// Example:
	//
	//     func(lstate *lua.LState) int {
	//     		env := lstate.CheckTable(1)
	//     		lstate.SetTable(env, lua.LString("key"), lua.LString("value"))
	//     		return 0
	//     	}
	//
	EnvFunc func(*lua.LState) int
}

// ExprOption is an option for [Expr].
type ExprOption func(*ExprConfig)

// WithDisableSandbox disables a sandboxing.
func WithDisableSandbox() ExprOption {
	return func(cfg *ExprConfig) {
		cfg.DisableSandbox = true
	}
}

// WithPoolSize is a size of the Lua VM pool.
// This defaults to 50.
// Pooling is disabled if size is a nevative value.
func WithPoolSize(size int) ExprOption {
	return func(cfg *ExprConfig) {
		cfg.PoolSize = size
	}
}

// WithAllowedFunctions is a space separated list of allowed Lua buitin functions.
// This defaults to [DefaultAllowedFunctions].
func WithAllowedFunctions(lst string) ExprOption {
	return func(cfg *ExprConfig) {
		cfg.AllowedFunctions = lst
	}
}

// WithEnvFunc is a function that customizes Lua environment.
// You can not use both of WithEnv and WithEnvFunc.
func WithEnvFunc(f func(*lua.LState) int) ExprOption {
	return func(cfg *ExprConfig) {
		cfg.EnvFunc = f
	}
}

// WithEnv is an [Env] that customizes Lua environment.
// You can not use both of WithEnv and WithEnvFunc.
func WithEnv(env Env) ExprOption {
	return func(cfg *ExprConfig) {
		cfg.EnvFunc = func(lstate *lua.LState) int {
			baseEnv := lstate.CheckTable(1)
			for key, value := range env {
				setTable(lstate, baseEnv, key, value)
			}
			return 0
		}
	}
}

// Expr is an interface that executes given expressions.
// Expr is a groutine safe.
type Expr interface {
	// Compile compiles a given expression.
	// Compiled expression can be cached and goroutine safe.
	Compile(expr string) (Evaler, error)

	// Close cleanups this object.
	Close()
}

// Env is an environment for evaluating expressions.
type Env map[string]interface{}

// Evaler is an interface that can be evaluatable.
type Evaler interface {
	// Eval evaluates the object with given environments.
	Eval(Env) (lua.LValue, error)

	// EvalBool evaluates the object as a boolean value with given environments.
	EvalBool(Env) (bool, error)

	// EvalContext evaluates the object with given environments.
	// Note that this function has a performance degradetion
	// compared with [Evaler].Eval.
	EvalContext(context.Context, Env) (lua.LValue, error)

	// EvalContextBool evaluates the object as a boolean value with given environments.
	// Note that this function has a performance degradetion
	// compared with [Evaler].EvalBool.
	EvalContextBool(context.Context, Env) (bool, error)
}

type evaler struct {
	sandbox bool
	proto   *lua.FunctionProto
	fn      *lua.LFunction
	lpool   lStatePool
}

func (e *evaler) eval(lstate *lua.LState, env Env) (lua.LValue, error) {
	if e.fn == nil {
		e.fn = lstate.NewFunctionFromProto(e.proto)
	} else {
		e.fn.Env = lstate.Env
	}
	if e.sandbox {
		ltbl := lstate.NewTable()
		for key, value := range env {
			setTable(lstate, ltbl, key, value)
		}
		if err := lstate.CallByParam(lua.P{
			Fn:      lstate.GetGlobal("sandbox_call"),
			NRet:    1,
			Protect: true,
		}, e.fn, ltbl); err != nil {
			return nil, fixError(err)
		}
	} else {
		if env != nil {
			ltbl := lstate.Get(lua.GlobalsIndex).(*lua.LTable)
			for key, value := range env {
				setTable(lstate, ltbl, key, value)
			}
		}
		if err := lstate.CallByParam(lua.P{
			Fn:      e.fn,
			NRet:    1,
			Protect: true,
		}); err != nil {
			return nil, fixError(err)
		}
	}
	ret := lstate.Get(-1)
	lstate.Pop(1)

	return ret, nil
}

func (e *evaler) Eval(env Env) (lua.LValue, error) {
	lstate := e.lpool.Get()
	defer e.lpool.Put(lstate)
	return e.eval(lstate, env)
}

func (e *evaler) EvalBool(env Env) (bool, error) {
	lv, err := e.Eval(env)
	if err != nil {
		return false, err
	}
	return lua.LVAsBool(lv), err
}

func (e *evaler) EvalContext(ctx context.Context, env Env) (lua.LValue, error) {
	lstate := e.lpool.Get()
	lstate.SetContext(ctx)
	defer func() {
		lstate.RemoveContext()
		e.lpool.Put(lstate)
	}()
	return e.eval(lstate, env)
}

func (e *evaler) EvalContextBool(ctx context.Context, env Env) (bool, error) {
	lv, err := e.EvalContext(ctx, env)
	if err != nil {
		return false, err
	}
	return lua.LVAsBool(lv), err
}

type expr struct {
	lpool lStatePool
	cfg   *ExprConfig
}

// New creates new [Expr]
func New(opts ...ExprOption) Expr {
	cfg := &ExprConfig{
		PoolSize:         50,
		AllowedFunctions: DefaultAllowedFunctions,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	factory := func() *lua.LState {
		lstate := lua.NewState(lua.Options{
			CallStackSize:       10,
			RegistrySize:        128,
			IncludeGoStackTrace: true,
		})
		if !cfg.DisableSandbox {
			if cfg.EnvFunc != nil {
				lstate.SetGlobal("__envfunc", lstate.NewFunction(cfg.EnvFunc))
			}
			err := lstate.DoString(strings.Replace(sandboxScript, "$$$", cfg.AllowedFunctions, 1))
			if err != nil {
				panic(err)
			}
		} else {
			if cfg.EnvFunc != nil {
				if err := lstate.CallByParam(lua.P{
					Fn:      lstate.NewFunction(cfg.EnvFunc),
					NRet:    0,
					Protect: true,
				}, lstate.Get(lua.GlobalsIndex)); err != nil {
					panic(err)
				}

			}
		}
		return lstate
	}
	var lpool lStatePool
	if cfg.PoolSize < 0 {
		lpool = newNocacheLStatePool(factory)
	} else {
		lpool = newSyncLStatePool(cfg.PoolSize, factory)
	}

	return &expr{
		lpool: lpool,
		cfg:   cfg,
	}
}

func (e *expr) Compile(expr string) (Evaler, error) {
	reader := strings.NewReader("return (" + expr + ")")
	chunk, err := parse.Parse(reader, "<glel>")
	if err != nil {
		return nil, fixError(err)
	}
	proto, err := lua.Compile(chunk, "<glel>")
	if err != nil {
		return nil, fixError(err)
	}
	proto.IsVarArg = 0
	return &evaler{sandbox: !e.cfg.DisableSandbox, proto: proto, lpool: e.lpool}, nil
}

func (e *expr) Close() {
	e.lpool.Shutdown()
}

func setTable(lstate *lua.LState, t *lua.LTable, key string, value interface{}) {
	if lv, ok := value.(lua.LValue); ok {
		lstate.SetTable(t, lua.LString(key), lv)
	} else {
		lstate.SetTable(t, lua.LString(key), luar.New(lstate, value))
	}
}

var pat *regexp.Regexp = regexp.MustCompile(`<string>:\d+:\s*`)

type eerror struct {
	lerr *lua.ApiError
}

func (err *eerror) Error() string {
	return pat.ReplaceAllString(err.lerr.Error(), "")
}

func fixError(err error) error {
	lerr, ok := err.(*lua.ApiError)
	if !ok {
		return err
	}
	return &eerror{lerr}
}
