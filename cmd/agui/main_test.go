package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRunArgs(t *testing.T) {
	file, shot, rest, debug, err := parseRunArgs([]string{"--shot=out.png", "app.ag", "one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	if file != "app.ag" || shot != "out.png" || len(rest) != 2 || rest[0] != "one" || debug {
		t.Fatalf("解析结果错误：file=%q shot=%q rest=%v debug=%v", file, shot, rest, debug)
	}
}

func TestParseRunArgsShotDefault(t *testing.T) {
	file, shot, _, _, err := parseRunArgs([]string{"app.ag", "--shot"})
	if err != nil {
		t.Fatal(err)
	}
	if file != "app.ag" || shot != "screenshot.png" {
		t.Fatalf("解析结果错误：file=%q shot=%q", file, shot)
	}
}

func TestParseRunArgsDebug(t *testing.T) {
	file, _, rest, debug, err := parseRunArgs([]string{"--debug", "app.ag", "--", "--help"})
	if err != nil {
		t.Fatal(err)
	}
	if file != "app.ag" || !debug || len(rest) != 1 || rest[0] != "--help" {
		t.Fatalf("解析结果错误：file=%q rest=%v debug=%v", file, rest, debug)
	}
}

func TestParseRunArgsMissingFile(t *testing.T) {
	if _, _, _, _, err := parseRunArgs([]string{"--shot"}); err == nil {
		t.Fatal("缺少脚本时应该报错")
	}
}

func TestParseRuntimeArgs(t *testing.T) {
	rest, shot, debug := parseRuntimeArgs([]string{"--shot=out.png", "--debug", "one", "--", "--two"})
	if shot != "out.png" || !debug || len(rest) != 2 || rest[0] != "one" || rest[1] != "--two" {
		t.Fatalf("解析结果错误：shot=%q debug=%v rest=%v", shot, debug, rest)
	}
}

func TestParseInitArgs(t *testing.T) {
	dir, name, err := parseInitArgs([]string{"demo", "--name", "我的应用"})
	if err != nil {
		t.Fatal(err)
	}
	if dir != "demo" || name != "我的应用" {
		t.Fatalf("解析结果错误：dir=%q name=%q", dir, name)
	}
}

func TestParseInitArgsDefaults(t *testing.T) {
	dir, name, err := parseInitArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if dir != "." || name != "" {
		t.Fatalf("解析结果错误：dir=%q name=%q", dir, name)
	}
}

func TestCollectScriptModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ag"), []byte("import \"lib/math\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib", "math.ag"), []byte("func inc(x):\n    return x+1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "app.exe")
	if err := os.WriteFile(out, []byte("exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	files, err := collectScriptModules(dir, out)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["main.ag"]; !ok {
		t.Fatal("缺少 main.ag")
	}
	if _, ok := files["lib/math.ag"]; !ok {
		t.Fatal("缺少 lib/math.ag")
	}
	if _, ok := files["notes.txt"]; ok {
		t.Fatal("不应收集非 .ag 文件")
	}
}

func TestParseBuildArgs(t *testing.T) {
	file, out, opts, err := parseBuildArgs([]string{"app.ag", "-o", "dist/app.exe", "-console", "--icon", "app.ico", "--product", "演示"})
	if err != nil {
		t.Fatal(err)
	}
	if file != "app.ag" || out != "dist/app.exe" || !opts.console || opts.icon != "app.ico" || opts.product != "演示" {
		t.Fatalf("解析结果错误：file=%q out=%q opts=%+v", file, out, opts)
	}
}

func TestParseBuildArgsOutputEquals(t *testing.T) {
	_, out, _, err := parseBuildArgs([]string{"app.ag", "--output=app.exe"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "app.exe" {
		t.Fatalf("out = %q", out)
	}
}

func TestParseBuildArgsVersion(t *testing.T) {
	_, _, opts, err := parseBuildArgs([]string{"app.ag", "--file-version=2.3.4.5", "--company=Agui"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.version != "2.3.4.5" || opts.company != "Agui" {
		t.Fatalf("opts = %+v", opts)
	}
}
