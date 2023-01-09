package glel

import (
	"strings"
	"testing"
	"time"

	"context"

	lua "github.com/yuin/gopher-lua"
)

func TestGlel(t *testing.T) {
	expr := New(
		WithAllowedFunctions(DefaultAllowedFunctions+`
	      string.rep
        `),
		WithPoolSize(-1),
		WithEnv(Env{
			"hoge": "foo",
		}))
	defer expr.Close()
	evaler, err := expr.Compile(`hoge == "foo" and add(x, y) == 15 and string.rep("ab", 5) == "ababababab" and d.name == "alice" `)
	if err != nil {
		t.Fatal(err.Error())
	}
	d := &struct {
		Name string
	}{
		Name: "alice",
	}
	result, err := evaler.EvalBool(Env{
		"y": 10,
		"x": lua.LNumber(5),
		"d": d,
		"add": func(a, b int) int {
			return a + b
		},
	})

	if err != nil {
		t.Fatal(err.Error())
	}

	if !result {
		t.Errorf("result should be true, but got %v", result)
	}

}

func TestGlelContext(t *testing.T) {
	expr := New(
		WithPoolSize(1),
		WithEnv(Env{
			"sleep": func() bool {
				time.Sleep(2 * time.Second)
				return true
			},
		}))
	defer expr.Close()
	evaler, err := expr.Compile(`sleep()`)
	if err != nil {
		t.Fatal(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	_, err = evaler.EvalContextBool(ctx, nil)

	if err == nil {
		t.Errorf("err should be occrred")
	} else if !strings.HasPrefix(err.Error(), "context deadline exceeded") {
		t.Errorf("err should be 'context deadline exceeded', but got '%s'.", err.Error())
	}

	// Pool size is 1, so this evaluation uses same lua.LState.
	result, err := evaler.EvalBool(nil)

	if err != nil {
		t.Fatal(err.Error())
	}

	if !result {
		t.Errorf("result should be true, but got %v", result)
	}

}
