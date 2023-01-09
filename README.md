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
        glel.WithAllowedFunctions(DefaultAllowedFunctions+`
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

`expr` is a goroutine safe. You can use one `expr` from multiple goroutines. `evaler` can be cached. Note that you **must not** closes `expr` before `evaler` evaluates.

License
--------------------
MIT

Author
--------------------
Yusuke Inuzuka
