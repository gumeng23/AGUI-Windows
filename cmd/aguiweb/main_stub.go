//go:build !js

package main

import "fmt"

func main() {
	fmt.Println("aguiweb 需要用 GOOS=js GOARCH=wasm 构建，然后在 WebView/浏览器中运行。")
}
