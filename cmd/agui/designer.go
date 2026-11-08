package main

import (
	"fmt"
	"os"
	"strings"

	"agui/internal/designer"
)

// runDesigner launches the embedded visual designer. An optional .ag file is
// passed to the designer so it can restore the saved graphical layout.
func runDesigner(args []string) {
	var file, shot string
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Println("用法：agui designer [设计文件.ag] [--shot=截图.png]")
			return
		}
		if strings.HasPrefix(a, "--shot=") {
			shot = strings.TrimPrefix(a, "--shot=")
			continue
		}
		if strings.HasPrefix(a, "-") {
			fmt.Fprintln(os.Stderr, "未知选项："+a)
			os.Exit(2)
		}
		if file == "" {
			file = a
		}
	}
	var extra []string
	if file != "" {
		if _, err := os.Stat(file); err != nil {
			fmt.Fprintln(os.Stderr, "错误：设计文件不存在："+file)
			os.Exit(2)
		}
		extra = []string{file}
	}
	runSource(designer.Source, "设计器", extra, shot, false, false, nil, "")
}
