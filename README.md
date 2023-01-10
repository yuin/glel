glel
==========================================

[![https://pkg.go.dev/github.com/yuin/glel](https://pkg.go.dev/badge/github.com/yuin/glel.svg)](https://pkg.go.dev/github.com/yuin/glel)

> Expression evaluation with Lua for Go

glel is an expression evaluation engine for Go using [GopherLua](http://github.com/yuin/gopher-lua)

## Features

- **Safe** : glel evaluates expressions in sandboxed environment.
- **Flexible** : You can write expressions in Lua, most widely used embedded language. 
- **Fast** : glel uses [GopherLua](http://github.com/yuin/gopher-lua) as a Lua interperter. GopherLua is a resonably fast for scripting and been used for many years in [many projects](https://pkg.go.dev/github.com/yuin/gopher-lua?tab=importedby)
- **Easy to use** : glel integrates Go and Lua using [gopher-luar](https://github.com/layeh/gopher-luar). Values are automatically converted for the languages.

## Usage

```go
    expr := glel.New(
        glel.WithAllowedFunctions(glel.DefaultAllowedFunctions+`
          string.rep
        `),
        glel.WithPoolSize(10),
        glel.WithEnv(glel.Env{
            "hoge": "foo",
        }))
    defer expr.Close()

    evaler, err := expr.Compile(`hoge == "foo" and add(x, y) == 15 and string.rep("ab", 5) == "ababababab" and d.name == "alice" `)
    if err != nil {
        panic(err)
    }

    d := &struct {
        Name string
    }{
        Name: "alice",
    }
    ok, err := evaler.EvalBool(glel.Env{
        "y": 10,
        "x": lua.LNumber(5),
        "d": d,
        "add": func(a, b int) int {
            return a + b
        },
    })
```

glel is a sandboxed by default. You can allow glel to use additonal builtin functions using `WithAllowedFunctions`.

glel pools Lua interpreters for performance. You can set pool size using `WithPoolSize`. glel does not pool Lua interpreters if `WithPoolSize` is set to negative.

`EvalContextXXX` behaves same as `EvalXXX`, but evaluates in coordination with `context`.

```go
    ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
    defer cancel()
    ok, err = evaler.EvalContextBool(ctx, nil)
```

`expr` is a goroutine safe if pooled( == WithPoolSize > 0 ). You can use one `expr` from multiple goroutines. `evaler` can be cached. Note that you **must not** closes `expr` before `evaler` evaluates.

Sandboxing has performance disadvantages.

```
Benchmark_noSandbox 218  ns/op      0 B/op   0 allocs/op
Benchmark_sandbox   3886 ns/op   4000 B/op  17 allocs/op
```

You can disable a sandboxing by `WithDisableSandbox()`.

## Benchmark
Modified version https://github.com/antonmedv/golang-expression-evaluation-comparison .

glel: WithDisableSandbox, WithPoolSize(-1)

```
Benchmark_bexpr-12                        461487              2462 ns/op             784 B/op         43 allocs/op
Benchmark_celgo-12                       5900113               201.4 ns/op            24 B/op          2 allocs/op
Benchmark_celgo_startswith-12            3268881               367.5 ns/op            88 B/op          6 allocs/op
Benchmark_evalfilter-12                   615469              1888 ns/op             736 B/op         21 allocs/op
Benchmark_expr-12                        8480037               139.7 ns/op            32 B/op          1 allocs/op
Benchmark_expr_startswith-12             3973664               301.0 ns/op           128 B/op          4 allocs/op
Benchmark_glel-12                        5481398               215.9 ns/op             0 B/op          0 allocs/op
Benchmark_goja-12                        3320126               362.4 ns/op            96 B/op          2 allocs/op
Benchmark_govaluate-12                   4233823               281.9 ns/op            24 B/op          2 allocs/op
Benchmark_gval-12                        1492868               803.0 ns/op           240 B/op          8 allocs/op
Benchmark_otto-12                        1415108               828.4 ns/op           336 B/op          7 allocs/op
Benchmark_starlark-12                     197761              6169 ns/op            3568 B/op         68 allocs/op
```

Althouh glel uses turing-complete language, is being resonably fast.


License
--------------------
MIT

Author
--------------------
Yusuke Inuzuka
