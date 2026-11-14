package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"agui/internal/ast"
	"agui/internal/gui"
	"agui/internal/lang"
	"agui/internal/lexer"
	"agui/internal/parser"
	"agui/internal/winres"
)

const version = "0.9.5"

// embedMagic marks a trailing script appended by `agui build`.
const embedMagic = "AGUIEMB\x01"
const embedMagic2 = "AGUIEMB"

// assetBundle is JSON stored in the v2 trailer.
type assetBundle struct {
	Files map[string]string `json:"files"`
}

func main() {
	if src, assets, baseDir, ok, err := embeddedScript(); err != nil {
		gui.ShowError("Agui 错误", "读取内置脚本失败："+err.Error())
		os.Exit(1)
	} else if ok {
		rest, shot, debug := parseRuntimeArgs(os.Args[1:])
		runSource(src, "内置脚本", rest, shot, true, debug, assets, baseDir)
		return
	}

	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		return
	}
	switch args[0] {
	case "designer":
		runDesigner(args[1:])
	case "run":
		file, shot, rest, debug, err := parseRunArgs(args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(2)
		}
		runFile(file, shot, rest, debug, false)
	case "check":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法：agui check <脚本.ag>")
			os.Exit(2)
		}
		checkFile(args[1])
	case "init":
		dir, name, err := parseInitArgs(args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(2)
		}
		initProject(dir, name)
	case "build":
		file, out, opts, err := parseBuildArgs(args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(2)
		}
		buildFile(file, out, opts)
	case "dist":
		if err := distApp(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "update":
		if err := runUpdate(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "install":
		rest := args[1:]
		registryURL := os.Getenv("AGUI_REGISTRY")
		force := false
		var positional []string
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case "--registry":
				if i+1 >= len(rest) {
					fmt.Fprintln(os.Stderr, "用法：agui install --registry <index.json> <包名>")
					os.Exit(2)
				}
				i++
				registryURL = rest[i]
			case "--force":
				force = true
			default:
				if strings.HasPrefix(rest[i], "-") {
					fmt.Fprintln(os.Stderr, "未知选项："+rest[i])
					os.Exit(2)
				}
				positional = append(positional, rest[i])
			}
		}
		if len(positional) < 1 {
			fmt.Fprintln(os.Stderr, "用法：agui install <包名[@版本]|URL|目录/文件> [包名] [--registry URL] [--force]")
			os.Exit(2)
		}
		name := ""
		if len(positional) >= 2 {
			name = positional[1]
		}
		if err := installPackageWith(positional[0], name, registryURL, force); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "list":
		if err := listPackages(); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "search":
		rest := args[1:]
		registryURL := os.Getenv("AGUI_REGISTRY")
		var words []string
		for i := 0; i < len(rest); i++ {
			if rest[i] == "--registry" && i+1 < len(rest) {
				i++
				registryURL = rest[i]
				continue
			}
			words = append(words, rest[i])
		}
		if err := searchPackages(strings.Join(words, " "), registryURL); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "outdated":
		registryURL := os.Getenv("AGUI_REGISTRY")
		for i := 1; i < len(args); i++ {
			if args[i] == "--registry" && i+1 < len(args) {
				i++
				registryURL = args[i]
			}
		}
		if err := outdatedPackages(registryURL); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "upgrade":
		rest := args[1:]
		registryURL := os.Getenv("AGUI_REGISTRY")
		force := false
		var refs []string
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case "--registry":
				if i+1 >= len(rest) {
					fmt.Fprintln(os.Stderr, "用法：agui upgrade [包名...] [--registry URL] [--force]")
					os.Exit(2)
				}
				i++
				registryURL = rest[i]
			case "--force":
				force = true
			default:
				refs = append(refs, rest[i])
			}
		}
		if err := upgradePackages(refs, registryURL, force); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法：agui remove <包名> [--force]")
			os.Exit(2)
		}
		force := false
		name := ""
		for _, a := range args[1:] {
			if a == "--force" {
				force = true
			} else if name == "" {
				name = a
			}
		}
		if name == "" {
			fmt.Fprintln(os.Stderr, "用法：agui remove <包名> [--force]")
			os.Exit(2)
		}
		if err := removePackage(name, force); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "debug":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法：agui debug <脚本.ag> [参数...]")
			os.Exit(2)
		}
		os.Setenv("AGUI_TRACE", "1")
		runFile(args[1], "", args[2:], true, false)
	case "watch":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法：agui watch <脚本.ag> [参数...]")
			os.Exit(2)
		}
		if err := watchFile(args[1], args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "lsp":
		if err := runLSP(); err != nil {
			fmt.Fprintln(os.Stderr, "LSP 错误："+err.Error())
			os.Exit(1)
		}
	case "verify":
		if err := verifyLock(); err != nil {
			fmt.Fprintln(os.Stderr, "错误："+err.Error())
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Printf("Agui %s\n", version)
	case "help", "-h", "--help":
		usage()
	default:
		if strings.HasSuffix(args[0], ".ag") {
			runFile(args[0], "", args[1:], false, false)
			return
		}
		fmt.Fprintf(os.Stderr, "未知命令：%s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`Agui - 用类 Python 的脚本写 Windows / Android 原生小程序

用法：
  agui run  <脚本.ag> [参数...]      运行脚本
  agui check <脚本.ag>               只做语法检查
  agui init [目录] [--name 名称]     生成一个可运行的示例项目
  agui designer [设计.ag] [--shot=图.png] 打开可视化 GUI 设计器（Windows）
  agui build <脚本.ag> [-o 输出]    按 #target 打包 exe 或 apk（默认 Windows exe）
  agui dist <脚本.ag>               生成 Windows 绿色包/安装脚本/update.json
  agui update check|apply <清单>    检查或应用自动更新（--current 当前exe）
  agui install <包名[@版本]>         从 registry 下载并安装包到 packages/
  agui install <源> [包名]           安装 .ag 包（URL/本地文件/目录）到 packages/
  --registry <index.json>           覆盖默认 registry 索引
  --force                           覆盖已安装的同名包
  agui list                          列出已安装的包
  agui search [关键词]               搜索 registry 中的包
  agui outdated                      列出有新版本的已安装包
  agui upgrade [包名...]             升级指定包或全部包
  agui remove <包名> [--force]       删除已安装的包；有反向依赖时需 --force
  agui verify                        按 agui.lock 校验包 SHA-256 与依赖
  agui watch <脚本.ag> [参数...]     文件变化时自动重启
  agui debug <脚本.ag> [参数...]     逐行跟踪（轻量调试器）
  agui lsp                           启动 LSP（stdio）
  agui version                       显示版本

运行选项：
  --shot[=文件.png]                  窗口显示后截图并退出（默认 screenshot.png）
  --debug                            调试模式：输出事件/调用日志，出错时打印全局变量

打包选项：
  -o <文件>                          输出文件名，默认与脚本同名（Windows .exe / Android .apk）
  --aab                              Android 构建为 .aab（App Bundle，未签名）
  -console                           强制复用当前控制台版本（默认优先找同目录 aguiw.exe）
  --asset <文件/目录>                嵌入资源，可重复；目录按目录名挂载
  --icon <图标>                      嵌入图标（Windows .ico / Android .png）
  --product <名称>                   产品名称（版本信息）
  --company <名称>                   公司名称
  --description <文字>               文件说明
  --copyright <文字>                 版权信息
  --file-version <1.2.3.4>           文件版本，默认 1.0.0.0
  --release                          Android release 构建（必须同时给 --ks/--ks-pass）
  --version-code <n>                 Android versionCode，默认 1

文件头（可选，仅入口文件）：
  #target android                    目标平台，缺省 windows
  #name 星野养成                      应用名称
  #package com.example.star          Android 包名
  #version 0.1.0                     版本
  #icon assets/icon.png              应用图标（相对脚本目录）

示例：
  agui init demo
  agui run examples/counter.ag
  agui build examples/counter.ag -o counter.exe --icon assets/app.ico --product 计数器
  agui build examples/android_star.ag -o star.apk
`)
}

func parseRuntimeArgs(args []string) (rest []string, shot string, debug bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--shot":
			shot = "screenshot.png"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && strings.HasSuffix(strings.ToLower(args[i+1]), ".png") {
				shot = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--shot="):
			shot = strings.TrimPrefix(a, "--shot=")
			if shot == "" {
				shot = "screenshot.png"
			}
		case a == "--debug" || a == "-d":
			debug = true
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		default:
			rest = append(rest, a)
		}
	}
	return rest, shot, debug
}

func parseRunArgs(args []string) (file, shot string, rest []string, debug bool, err error) {
	positionals, shot, debug := parseRuntimeArgs(args)
	if len(positionals) == 0 {
		return "", "", nil, false, fmt.Errorf("缺少脚本文件")
	}
	return positionals[0], shot, positionals[1:], debug, nil
}

func parseInitArgs(args []string) (dir, name string, err error) {
	dir = "."
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--name" || a == "-n":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s 后面需要名称", a)
			}
			name = args[i+1]
			i++
		case strings.HasPrefix(a, "--name="):
			name = strings.TrimPrefix(a, "--name=")
		case strings.HasPrefix(a, "-"):
			return "", "", fmt.Errorf("未知参数：%s", a)
		default:
			if dir != "." {
				return "", "", fmt.Errorf("只能指定一个目录")
			}
			dir = a
		}
	}
	return dir, name, nil
}

type buildOptions struct {
	icon        string
	assets      []string
	product     string
	company     string
	description string
	copyright   string
	version     string
	console     bool
	release     bool
	ks          string
	ksPass      string
	keyAlias    string
	keyPass     string
	versionCode int
	aab         bool
}

func parseBuildArgs(args []string) (file, out string, opts buildOptions, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func(name string) (string, bool) {
			if i+1 >= len(args) {
				err = fmt.Errorf("%s 后面需要参数", name)
				return "", false
			}
			i++
			return args[i], true
		}
		switch {
		case a == "-o" || a == "--output":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			out = v
		case strings.HasPrefix(a, "-o="):
			out = strings.TrimPrefix(a, "-o=")
		case strings.HasPrefix(a, "--output="):
			out = strings.TrimPrefix(a, "--output=")
		case a == "-console" || a == "--console":
			opts.console = true
		case a == "--release":
			opts.release = true
		case a == "--aab":
			opts.aab = true
		case a == "--ks":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.ks = v
		case strings.HasPrefix(a, "--ks="):
			opts.ks = strings.TrimPrefix(a, "--ks=")
		case a == "--ks-pass":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.ksPass = v
		case strings.HasPrefix(a, "--ks-pass="):
			opts.ksPass = strings.TrimPrefix(a, "--ks-pass=")
		case a == "--key-alias":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.keyAlias = v
		case strings.HasPrefix(a, "--key-alias="):
			opts.keyAlias = strings.TrimPrefix(a, "--key-alias=")
		case a == "--key-pass":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.keyPass = v
		case strings.HasPrefix(a, "--key-pass="):
			opts.keyPass = strings.TrimPrefix(a, "--key-pass=")
		case a == "--version-code":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			if n, e := strconv.Atoi(v); e == nil {
				opts.versionCode = n
			}
		case strings.HasPrefix(a, "--version-code="):
			if n, e := strconv.Atoi(strings.TrimPrefix(a, "--version-code=")); e == nil {
				opts.versionCode = n
			}
		case a == "--asset":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.assets = append(opts.assets, v)
		case strings.HasPrefix(a, "--asset="):
			opts.assets = append(opts.assets, strings.TrimPrefix(a, "--asset="))
		case a == "--icon":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.icon = v
		case strings.HasPrefix(a, "--icon="):
			opts.icon = strings.TrimPrefix(a, "--icon=")
		case a == "--product":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.product = v
		case strings.HasPrefix(a, "--product="):
			opts.product = strings.TrimPrefix(a, "--product=")
		case a == "--company":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.company = v
		case strings.HasPrefix(a, "--company="):
			opts.company = strings.TrimPrefix(a, "--company=")
		case a == "--description":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.description = v
		case strings.HasPrefix(a, "--description="):
			opts.description = strings.TrimPrefix(a, "--description=")
		case a == "--copyright":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.copyright = v
		case strings.HasPrefix(a, "--copyright="):
			opts.copyright = strings.TrimPrefix(a, "--copyright=")
		case a == "--file-version":
			v, ok := next(a)
			if !ok {
				return "", "", opts, err
			}
			opts.version = v
		case strings.HasPrefix(a, "--file-version="):
			opts.version = strings.TrimPrefix(a, "--file-version=")
		case file == "" && !strings.HasPrefix(a, "-"):
			file = a
		default:
			return "", "", opts, fmt.Errorf("未知参数：%s", a)
		}
	}
	if file == "" {
		return "", "", opts, fmt.Errorf("缺少脚本文件")
	}
	return file, out, opts, nil
}

func runFile(path, shot string, extra []string, debug bool, embedded bool) {
	src, err := os.ReadFile(path)
	if err != nil {
		fail(fmt.Errorf("无法读取脚本：%w", err), nil, embedded)
		os.Exit(1)
	}
	baseDir := filepath.Dir(path)
	if abs, absErr := filepath.Abs(path); absErr == nil {
		baseDir = filepath.Dir(abs)
	}
	runSource(string(src), path, extra, shot, embedded, debug, nil, baseDir)
}

func runSource(src, name string, extra []string, shot string, embedded bool, debug bool, assets map[string][]byte, baseDir string) {
	if debug {
		os.Setenv("AGUI_DEBUG", "1")
		fmt.Fprintln(os.Stderr, "[debug] 调试模式已开启")
	}
	prog, err := parseSource(src)
	if err != nil {
		text := fmt.Sprintf("%s：%v", name, err)
		if embedded {
			gui.ShowError("Agui 语法错误", text)
		} else {
			fmt.Fprintln(os.Stderr, text)
		}
		os.Exit(1)
	}

	i := lang.New()
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}
	i.SetAssets(assets, baseDir)
	i.SetBaseDir(baseDir)
	app, err := registerGUI(i)
	if err != nil {
		fail(fmt.Errorf("初始化 GUI 失败：%w", err), nil, embedded)
		os.Exit(1)
	}
	i.SetSource(name, src)
	if os.Getenv("AGUI_TRACE") == "1" {
		fmt.Fprintf(os.Stderr, "[debug] 跟踪 %s\n", filepath.Base(name))
		i.SetTrace(func(line int) {
			if line >= 1 && line <= len(i.Lines) {
				fmt.Fprintf(os.Stderr, "[debug] %s:%d  %s\n", filepath.Base(name), line, strings.TrimSpace(i.Lines[line-1]))
			}
		})
	}

	vals := make([]lang.Value, 0, len(extra))
	for _, a := range extra {
		vals = append(vals, a)
	}
	i.Define("args", lang.NewList(vals...))
	i.Define("script_path", name)
	if shot != "" {
		app.SetStartupShot(shot)
	}

	if err := i.Execute(prog); err != nil {
		fail(err, i, embedded)
		if debug {
			dumpGlobals(i)
		}
		os.Exit(1)
	}
	if !app.HasWindow() || app.Ran() {
		return
	}
	if err := app.Run(); err != nil {
		fail(err, i, embedded)
		if debug {
			dumpGlobals(i)
		}
		os.Exit(1)
	}
}

func initProject(dir, name string) {
	if name == "" {
		name = "Agui 应用"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "错误："+err.Error())
		os.Exit(1)
	}
	path := filepath.Join(dir, "main.ag")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "错误：%s 已存在，未覆盖\n", path)
		os.Exit(1)
	}
	template := fmt.Sprintf(`# %s
# 用 agui run main.ag 运行，用 agui build main.ag 打包。

window("%s", 720, 520)

title_font = font("Microsoft YaHei UI", 22, bold=true)
label("你好，Agui！", font=title_font)
label("这是一个可以继续修改的示例。", fg="#64748B")

count = 0
result = label("点击次数：0", y=20)

add = button("点我 +1")
on add.click:
    count += 1
    result.text = "点击次数：" + str(count)
    if count >= 10:
        result.text = "已经点了 10 次！"

edit = edit("", y=20)
on edit.change:
    result.text = "输入内容：" + edit.text

row = checkbox("记住我", y=20)
on row.change:
    result.text = "记住我：" + str(row.checked)

status = label("快捷键：Ctrl+O 打开文件", y=20, fg="#64748B")

menu = menu("文件")
menu.item("打开", lambda: msgbox("提示", "这里可以放打开文件的代码"), accel="Ctrl+O")
menu.item("退出", lambda: quit(), accel="Alt+F4")

run()
`, name, name)
	if err := os.WriteFile(path, []byte(template), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "错误："+err.Error())
		os.Exit(1)
	}
	fmt.Printf("已生成 %s\n运行：agui run %s\n", path, path)
}

func dumpGlobals(i *lang.Interp) {
	names := make([]string, 0)
	values := i.GlobalsSnapshot()
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Fprintln(os.Stderr, "--- 全局变量 ---")
	for _, name := range names {
		switch values[name].(type) {
		case *lang.Native:
			continue
		}
		text := lang.ToString(values[name])
		if len([]rune(text)) > 120 {
			text = string([]rune(text)[:120]) + "..."
		}
		fmt.Fprintf(os.Stderr, "  %s = %s\n", name, text)
	}
}

func parseSource(src string) (*ast.Program, error) {
	toks, err := lexer.Lex(src)
	if err != nil {
		return nil, err
	}
	return parser.Parse(toks)
}

func registerGUI(i *lang.Interp) (app *gui.App, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	app = gui.Register(i)
	return app, nil
}

func checkFile(path string) {
	_, src, err := readTargetHeader(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := parseSource(src); err != nil {
		fmt.Fprintf(os.Stderr, "%s：%v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("✓ 语法检查通过：%s\n", path)
}

func fail(err error, i *lang.Interp, embedded bool) {
	text := err.Error()
	if i != nil {
		text = i.FormatError(err)
	}
	if embedded && os.Getenv("AGUI_STDERR") == "" {
		gui.ShowError("Agui 运行错误", text)
		return
	}
	fmt.Fprintln(os.Stderr, text)
}

func embeddedScript() (string, map[string][]byte, string, bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", nil, "", false, nil
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return "", nil, "", false, nil
	}
	baseDir := filepath.Dir(exe)
	if len(data) < 8 {
		return "", nil, "", false, nil
	}
	magic := string(data[len(data)-8:])
	switch magic {
	case embedMagic:
		if len(data) < 16 {
			return "", nil, "", false, fmt.Errorf("内置脚本长度无效")
		}
		n := binary.LittleEndian.Uint64(data[len(data)-16 : len(data)-8])
		if n > uint64(len(data)-16) {
			return "", nil, "", false, fmt.Errorf("内置脚本长度无效")
		}
		start := len(data) - 16 - int(n)
		return string(data[start : len(data)-16]), nil, baseDir, true, nil
	case embedMagic2:
		if len(data) < 32 {
			return "", nil, "", false, fmt.Errorf("内置脚本长度无效")
		}
		scriptLen := binary.LittleEndian.Uint64(data[len(data)-24 : len(data)-16])
		assetsLen := binary.LittleEndian.Uint64(data[len(data)-16 : len(data)-8])
		if scriptLen > uint64(len(data)-32) || assetsLen > uint64(len(data)-32)-scriptLen {
			return "", nil, "", false, fmt.Errorf("内置脚本长度无效")
		}
		start := len(data) - 24 - int(scriptLen) - int(assetsLen)
		src := string(data[start : start+int(scriptLen)])
		assets := map[string][]byte{}
		if assetsLen > 0 {
			var bundle assetBundle
			if err := json.Unmarshal(data[start+int(scriptLen):start+int(scriptLen)+int(assetsLen)], &bundle); err != nil {
				return "", nil, "", false, fmt.Errorf("内置资源损坏: %v", err)
			}
			for name, enc := range bundle.Files {
				raw, err := base64.StdEncoding.DecodeString(enc)
				if err != nil {
					return "", nil, "", false, fmt.Errorf("内置资源 %s 损坏: %v", name, err)
				}
				assets[name] = raw
			}
		}
		return src, assets, baseDir, true, nil
	default:
		return "", nil, "", false, nil
	}
}

// collectAssets reads files/directories into a virtual path map. A directory
// is mounted under its base name, so --asset assets puts assets/logo.png at
// the script-visible path "assets/logo.png".
// collectScriptModules gathers .ag files next to the entry script so packaged
// applications can import sibling modules without extra --asset flags.
func collectScriptModules(dir, skip string) (map[string][]byte, error) {
	out := map[string][]byte{}
	skipAbs, _ := filepath.Abs(skip)
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() || !strings.EqualFold(filepath.Ext(p), ".ag") {
			return nil
		}
		if abs, _ := filepath.Abs(p); abs == skipAbs {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = raw
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func collectAssets(paths []string, skip string) (map[string][]byte, error) {
	out := map[string][]byte{}
	skipAbs, _ := filepath.Abs(skip)
	for _, root := range paths {
		info, err := os.Stat(root)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", root, err)
		}
		if !info.IsDir() {
			raw, err := os.ReadFile(root)
			if err != nil {
				return nil, err
			}
			if abs, _ := filepath.Abs(root); abs == skipAbs {
				continue
			}
			out[filepath.ToSlash(filepath.Base(root))] = raw
			continue
		}
		prefix := filepath.Base(filepath.Clean(root))
		if prefix == "." || prefix == string(filepath.Separator) {
			prefix = ""
		}
		err = filepath.Walk(root, func(p string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if fi.IsDir() {
				return nil
			}
			if abs, _ := filepath.Abs(p); abs == skipAbs {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			key := filepath.ToSlash(rel)
			if prefix != "" {
				key = prefix + "/" + key
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out[key] = raw
			if prefix != "" {
				if _, exists := out[filepath.ToSlash(rel)]; !exists {
					out[filepath.ToSlash(rel)] = raw
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", root, err)
		}
	}
	return out, nil
}

func buildFile(path, out string, opts buildOptions) {
	info, srcText, err := readTargetHeader(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	src := []byte(srcText)
	if _, err := parseSource(string(src)); err != nil {
		fmt.Fprintf(os.Stderr, "%s：%v\n", path, err)
		os.Exit(1)
	}
	if info.OS == "android" {
		if out == "" {
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			if opts.aab {
				out = base + ".aab"
			} else {
				out = base + ".apk"
			}
		}
		buildAndroid(path, string(src), info, out, opts)
		return
	}
	if out == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		out = base + ".exe"
	}
	out, err = filepath.Abs(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "输出路径无效：%v\n", err)
		os.Exit(1)
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法定位 agui.exe：%v\n", err)
		os.Exit(1)
	}
	baseExe := exe
	if !opts.console {
		guiExe := filepath.Join(filepath.Dir(exe), "aguiw.exe")
		if _, statErr := os.Stat(guiExe); statErr == nil {
			baseExe = guiExe
		} else {
			fmt.Fprintln(os.Stderr, "提示：同目录未找到 aguiw.exe，输出文件仍会带控制台窗口")
		}
	}
	if same, _ := filepath.Abs(baseExe); same == out {
		fmt.Fprintln(os.Stderr, "错误：输出文件不能覆盖运行器本身")
		os.Exit(1)
	}
	data, err := os.ReadFile(baseExe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法读取运行器：%v\n", err)
		os.Exit(1)
	}
	var icon []byte
	if opts.icon != "" {
		icon, err = os.ReadFile(opts.icon)
		if err != nil {
			fmt.Fprintf(os.Stderr, "无法读取图标：%v\n", err)
			os.Exit(1)
		}
	}
	baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if opts.product == "" {
		opts.product = baseName
	}
	if opts.description == "" {
		opts.description = opts.product
	}
	if opts.version == "" {
		opts.version = "1.0.0.0"
	}
	data, err = winres.AddResources(data, winres.Options{
		Icon:            icon,
		CompanyName:     opts.company,
		ProductName:     opts.product,
		FileDescription: opts.description,
		FileVersion:     opts.version,
		ProductVersion:  opts.version,
		LegalCopyright:  opts.copyright,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "写入图标/版本资源失败：%v\n", err)
		os.Exit(1)
	}
	assetFiles, err := collectScriptModules(filepath.Dir(path), out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取模块失败：%v\n", err)
		os.Exit(1)
	}
	explicit, err := collectAssets(opts.assets, out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取资源失败：%v\n", err)
		os.Exit(1)
	}
	for name, raw := range explicit {
		assetFiles[name] = raw
	}
	bundle := assetBundle{Files: map[string]string{}}
	for name, raw := range assetFiles {
		bundle.Files[name] = base64.StdEncoding.EncodeToString(raw)
	}
	assetsJSON, err := json.Marshal(bundle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打包资源失败：%v\n", err)
		os.Exit(1)
	}
	data = append(data, []byte(src)...)
	data = append(data, assetsJSON...)
	var lens [16]byte
	binary.LittleEndian.PutUint64(lens[0:8], uint64(len(src)))
	binary.LittleEndian.PutUint64(lens[8:16], uint64(len(assetsJSON)))
	data = append(data, lens[:]...)
	data = append(data, []byte(embedMagic2)...)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "无法创建输出目录：%v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, data, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "写入失败：%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 已生成 %s（%d 字节）\n", out, len(data))
}
