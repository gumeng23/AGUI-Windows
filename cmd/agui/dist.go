package main

// Packaging for portable Windows releases and a small manifest based updater.

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type distOptions struct {
	file    string
	outDir  string
	name    string
	version string
	url     string
	notes   string
	build   []string
}

type updateManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Notes   string `json:"notes"`
}

func parseDistArgs(args []string) (distOptions, error) {
	var opt distOptions
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func(flag string) (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s 后面需要值", flag)
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "-o" || a == "--output":
			v, err := next(a)
			if err != nil {
				return opt, err
			}
			opt.outDir = v
		case strings.HasPrefix(a, "--output="):
			opt.outDir = strings.TrimPrefix(a, "--output=")
		case a == "--name":
			v, err := next(a)
			if err != nil {
				return opt, err
			}
			opt.name = v
		case strings.HasPrefix(a, "--name="):
			opt.name = strings.TrimPrefix(a, "--name=")
		case a == "--version":
			v, err := next(a)
			if err != nil {
				return opt, err
			}
			opt.version = v
		case strings.HasPrefix(a, "--version="):
			opt.version = strings.TrimPrefix(a, "--version=")
		case a == "--url":
			v, err := next(a)
			if err != nil {
				return opt, err
			}
			opt.url = v
		case strings.HasPrefix(a, "--url="):
			opt.url = strings.TrimPrefix(a, "--url=")
		case a == "--notes":
			v, err := next(a)
			if err != nil {
				return opt, err
			}
			opt.notes = v
		case strings.HasPrefix(a, "--notes="):
			opt.notes = strings.TrimPrefix(a, "--notes=")
		case opt.file == "" && !strings.HasPrefix(a, "-"):
			opt.file = a
		default:
			opt.build = append(opt.build, a)
		}
	}
	if opt.file == "" {
		return opt, fmt.Errorf("缺少脚本文件")
	}
	if opt.outDir == "" {
		opt.outDir = "dist"
	}
	return opt, nil
}

func distApp(args []string) error {
	opt, err := parseDistArgs(args)
	if err != nil {
		return err
	}
	info, _, err := readTargetHeader(opt.file)
	if err != nil {
		return err
	}
	if info.OS == "android" {
		return fmt.Errorf("dist 目前只支持 Windows 目标；Android 请用 agui build")
	}
	if opt.name == "" {
		opt.name = info.Name
	}
	if opt.name == "" {
		opt.name = strings.TrimSuffix(filepath.Base(opt.file), filepath.Ext(opt.file))
	}
	if opt.version == "" {
		opt.version = info.Version
	}
	if opt.version == "" {
		opt.version = "1.0.0"
	}
	opt.url = strings.TrimSpace(opt.url)
	if opt.url == "" {
		opt.url = "https://example.com/" + safeFileName(opt.name) + "-" + opt.version + "-win64.zip"
	}

	safe := safeFileName(opt.name)
	appDir := filepath.Join(opt.outDir, safe)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return err
	}
	exePath := filepath.Join(appDir, safe+".exe")
	buildArgs := []string{opt.file, "-o", exePath, "--product", opt.name, "--file-version", opt.version}
	buildArgs = append(buildArgs, opt.build...)
	file, out, opts, err := parseBuildArgs(buildArgs)
	if err != nil {
		return err
	}
	buildFile(file, out, opts)

	exe, err := os.ReadFile(exePath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(exe)
	manifest := updateManifest{
		Name:    opt.name,
		Version: opt.version,
		URL:     opt.url,
		SHA256:  hex.EncodeToString(sum[:]),
		Notes:   opt.notes,
	}
	man, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(appDir, "update.json"), append(man, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(appDir, "install.ps1"), []byte(installScript(opt.name, safe)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(appDir, "uninstall.ps1"), []byte(uninstallScript(safe)), 0o644); err != nil {
		return err
	}
	readme := fmt.Sprintf("%s %s\r\n\r\n1. 解压到任意目录。\r\n2. 右键 install.ps1 -> 使用 PowerShell 运行。\r\n3. 卸载运行 uninstall.ps1。\r\n\r\n更新清单：update.json\r\n", opt.name, opt.version)
	if err := os.WriteFile(filepath.Join(appDir, "README.txt"), []byte(readme), 0o644); err != nil {
		return err
	}
	zipPath := filepath.Join(opt.outDir, safe+"-"+opt.version+"-win64.zip")
	if err := zipDir(appDir, zipPath); err != nil {
		return err
	}
	fmt.Printf("✓ 已生成 %s\n", zipPath)
	fmt.Printf("  安装包目录：%s\n", appDir)
	fmt.Printf("  SHA-256：%s\n", manifest.SHA256)
	return nil
}

func safeFileName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "app"
	}
	var b strings.Builder
	for _, r := range s {
		if r == '\\' || r == '/' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r == ' ' {
			b.WriteRune('-')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func psQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func installScript(name, safe string) string {
	qName := psQuote(name)
	qSafe := psQuote(safe)
	return "$ErrorActionPreference = 'Stop'\r\n" +
		"$src = $PSScriptRoot\r\n" +
		"$dest = Join-Path $env:LOCALAPPDATA 'AguiApps\\" + qSafe + "'\r\n" +
		"New-Item -ItemType Directory -Force $dest | Out-Null\r\n" +
		"Copy-Item -Path (Join-Path $src '*') -Destination $dest -Recurse -Force\r\n" +
		"$exe = Join-Path $dest '" + qSafe + ".exe'\r\n" +
		"$desktop = [Environment]::GetFolderPath('Desktop')\r\n" +
		"$link = Join-Path $desktop '" + qName + ".lnk'\r\n" +
		"$ws = New-Object -ComObject WScript.Shell\r\n" +
		"$sc = $ws.CreateShortcut($link)\r\n" +
		"$sc.TargetPath = $exe\r\n" +
		"$sc.WorkingDirectory = $dest\r\n" +
		"$sc.Save()\r\n" +
		"Write-Host ('已安装到 ' + $dest)\r\n"
}

func uninstallScript(safe string) string {
	qSafe := psQuote(safe)
	return "$ErrorActionPreference = 'Stop'\r\n" +
		"$dest = Join-Path $env:LOCALAPPDATA 'AguiApps\\" + qSafe + "'\r\n" +
		"$desktop = [Environment]::GetFolderPath('Desktop')\r\n" +
		"$link = Join-Path $desktop '" + psQuote(safe) + ".lnk'\r\n" +
		"if (Test-Path $link) { Remove-Item -Force $link }\r\n" +
		"if (Test-Path $dest) { Remove-Item -Recurse -Force $dest }\r\n" +
		"Write-Host '已卸载'\r\n"
}

func zipDir(dir, out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	base := filepath.Base(dir)
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(filepath.Join(base, rel)))
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
}

func runUpdate(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法：agui update check|apply <manifest.json或URL> [--current 当前exe]")
	}
	action := args[0]
	source := args[1]
	current := ""
	for i := 2; i < len(args); i++ {
		if args[i] == "--current" && i+1 < len(args) {
			i++
			current = args[i]
		} else if strings.HasPrefix(args[i], "--current=") {
			current = strings.TrimPrefix(args[i], "--current=")
		}
	}
	data, base, err := fetchUpdateSource(source)
	if err != nil {
		return err
	}
	var m updateManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("解析更新清单：%w", err)
	}
	if m.Name == "" || m.Version == "" || m.URL == "" {
		return fmt.Errorf("更新清单缺少 name/version/url")
	}
	switch action {
	case "check":
		fmt.Printf("应用：%s\n版本：%s\n地址：%s\n", m.Name, m.Version, m.URL)
		if m.Notes != "" {
			fmt.Printf("说明：%s\n", m.Notes)
		}
		if current != "" {
			fmt.Printf("当前文件：%s\n", current)
		}
		return nil
	case "apply":
		if current == "" {
			return fmt.Errorf("apply 需要 --current <当前 exe>")
		}
		if !strings.HasSuffix(strings.ToLower(current), ".exe") {
			return fmt.Errorf("--current 必须是 .exe")
		}
		raw, err := fetchUpdateBytes(m.URL, base)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		got := hex.EncodeToString(sum[:])
		if m.SHA256 != "" && !strings.EqualFold(got, m.SHA256) {
			return fmt.Errorf("SHA-256 不匹配：期望 %s，实际 %s", m.SHA256, got)
		}
		if strings.HasSuffix(strings.ToLower(m.URL), ".zip") {
			raw, err = extractUpdateExe(raw)
			if err != nil {
				return err
			}
		}
		next := current + ".new"
		if err := os.WriteFile(next, raw, 0o755); err != nil {
			return err
		}
		ps := current + ".update.ps1"
		script := "Start-Sleep -Milliseconds 600\r\n" +
			"Copy-Item -LiteralPath '" + psQuote(next) + "' -Destination '" + psQuote(current) + "' -Force\r\n" +
			"Start-Process -FilePath '" + psQuote(current) + "'\r\n" +
			"Remove-Item -LiteralPath $MyInvocation.MyCommand.Path -Force\r\n"
		if err := os.WriteFile(ps, []byte(script), 0o644); err != nil {
			return err
		}
		cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", ps)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("启动更新脚本：%w", err)
		}
		fmt.Printf("✓ 已下载 %s（%d 字节），更新脚本已启动\n", next, len(raw))
		return nil
	default:
		return fmt.Errorf("未知 update 操作：%s", action)
	}
}

func extractUpdateExe(raw []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("更新包不是有效 zip：%w", err)
	}
	var best *zip.File
	for _, f := range zr.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".exe") {
			continue
		}
		if best == nil || f.UncompressedSize64 > best.UncompressedSize64 {
			best = f
		}
	}
	if best == nil {
		return nil, fmt.Errorf("更新包中没有 .exe")
	}
	rc, err := best.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func fetchUpdateSource(source string) ([]byte, string, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, err := http.Get(source)
		if err != nil {
			return nil, source, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, source, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		return data, source, err
	}
	data, err := os.ReadFile(source)
	abs, _ := filepath.Abs(source)
	return data, "file://" + abs, err
}

func fetchUpdateBytes(url, base string) ([]byte, error) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}
	path := strings.TrimPrefix(url, "file://")
	if path == url {
		if strings.HasPrefix(base, "file://") {
			path = filepath.Join(filepath.Dir(strings.TrimPrefix(base, "file://")), url)
		}
	}
	return os.ReadFile(path)
}
