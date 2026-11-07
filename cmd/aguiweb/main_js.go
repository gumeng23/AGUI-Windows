//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"agui/internal/lang"
	"agui/internal/lexer"
	"agui/internal/parser"
	"agui/internal/webui"
)

func main() {
	app := webui.Register(lang.New())
	i := app.Interp()
	src := loadScript("game.ag")
	if src == "" {
		app.Fatal(fmt.Errorf("无法加载 game.ag"))
		return
	}
	i.SetSource("game.ag", src)
	toks, err := lexer.Lex(src)
	if err != nil {
		app.Fatal(err)
		return
	}
	prog, err := parser.Parse(toks)
	if err != nil {
		app.Fatal(err)
		return
	}
	i.Define("args", lang.NewList())
	i.Define("script_path", "game.ag")
	if err := i.Execute(prog); err != nil {
		app.Fatal(err)
		return
	}
	<-app.Done()
}

func loadScript(path string) string {
	if v := js.Global().Get("__AGUI_SOURCE__"); v.Type() == js.TypeString {
		return v.String()
	}
	xhr := js.Global().Get("XMLHttpRequest").New()
	xhr.Call("open", "GET", path, false)
	xhr.Call("send")
	if xhr.Get("status").Int() != 200 {
		return ""
	}
	return xhr.Get("responseText").String()
}
