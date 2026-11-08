package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type packageMeta struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Source       string            `json:"source"`
	Entry        string            `json:"entry,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

type lockEntry struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Source       string            `json:"source"`
	Entry        string            `json:"entry"`
	SHA256       string            `json:"sha256"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

type lockFile struct {
	Version  int         `json:"version"`
	Packages []lockEntry `json:"packages"`
}

func packagesDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "packages"
	}
	return filepath.Join(wd, "packages")
}

func safePackageName(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".ag")
	s = strings.Trim(s, "/\\")
	if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/:") || strings.IndexByte(s, 92) >= 0 {
		return "", fmt.Errorf("包名不合法：%q", s)
	}
	return s, nil
}

func packageNameFromSource(src string) string {
	s := strings.TrimSuffix(src, "/")
	if i := strings.LastIndexAny(s, "/\\"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".ag")
	return s
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func downloadFile(url string, limit int64) ([]byte, error) {
	data, err := registryGET(url, limit)
	if err != nil {
		return nil, fmt.Errorf("下载失败：%w", err)
	}
	return data, nil
}

func installPackage(src, nameArg string) error {
	if strings.TrimSpace(src) == "" {
		return fmt.Errorf("需要 URL 或本地目录/文件")
	}
	name := nameArg
	if name == "" {
		name = packageNameFromSource(src)
	}
	clean, err := safePackageName(name)
	if err != nil {
		return err
	}
	root := packagesDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	meta := packageMeta{Name: clean, Version: "0.1.0", Source: src}
	status := "已安装"
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		if !strings.HasSuffix(strings.ToLower(src), ".ag") {
			return fmt.Errorf("URL 安装目前只支持直接指向 .ag 文件")
		}
		data, err := downloadFile(src, 20<<20)
		if err != nil {
			return err
		}
		entry := filepath.Join(root, clean+".ag")
		if err := os.WriteFile(entry, data, 0o644); err != nil {
			return err
		}
		meta.Entry = clean + ".ag"
	} else {
		st, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("找不到包源 %q：%v", src, err)
		}
		if st.IsDir() {
			dst := filepath.Join(root, clean)
			if _, err := os.Stat(dst); err == nil {
				return fmt.Errorf("包 %q 已存在，请先 remove", clean)
			}
			if err := copyDir(src, dst); err != nil {
				return err
			}
			for _, cand := range []string{clean + ".ag", "index.ag", "main.ag"} {
				if _, err := os.Stat(filepath.Join(dst, cand)); err == nil {
					meta.Entry = filepath.Join(clean, cand)
					break
				}
			}
			if meta.Entry == "" {
				entries, _ := os.ReadDir(dst)
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".ag") {
						meta.Entry = filepath.Join(clean, e.Name())
						break
					}
				}
			}
			if meta.Entry == "" {
				_ = os.RemoveAll(dst)
				return fmt.Errorf("目录中没有 .ag 入口文件")
			}
		} else {
			if !strings.HasSuffix(strings.ToLower(src), ".ag") {
				return fmt.Errorf("包文件必须是 .ag")
			}
			entry := filepath.Join(root, clean+".ag")
			if err := copyFile(src, entry); err != nil {
				return err
			}
			meta.Entry = clean + ".ag"
		}
	}
	mb, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(root, clean+".json"), mb, 0o644); err != nil {
		return err
	}
	if err := addLockEntry(meta); err != nil {
		return err
	}
	fmt.Printf("%s 包 %s（入口 %s）\n", status, clean, meta.Entry)
	return nil
}

func lockPath() string { return "agui.lock" }

func addLockEntry(meta packageMeta) error {
	entryPath := filepath.Join(packagesDir(), filepath.FromSlash(meta.Entry))
	data, err := os.ReadFile(entryPath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	entry := lockEntry{Name: meta.Name, Version: meta.Version, Source: meta.Source, Entry: meta.Entry, SHA256: hex.EncodeToString(sum[:]), Dependencies: meta.Dependencies}
	lf := lockFile{Version: 1}
	if b, err := os.ReadFile(lockPath()); err == nil {
		_ = json.Unmarshal(b, &lf)
	}
	found := false
	for i := range lf.Packages {
		if lf.Packages[i].Name == entry.Name {
			lf.Packages[i] = entry
			found = true
		}
	}
	if !found {
		lf.Packages = append(lf.Packages, entry)
	}
	sort.Slice(lf.Packages, func(i, j int) bool { return lf.Packages[i].Name < lf.Packages[j].Name })
	b, _ := json.MarshalIndent(lf, "", "  ")
	return os.WriteFile(lockPath(), append(b, '\n'), 0o644)
}

func verifyLock() error {
	b, err := os.ReadFile(lockPath())
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("没有 agui.lock")
			return nil
		}
		return err
	}
	var lf lockFile
	if err := json.Unmarshal(b, &lf); err != nil {
		return fmt.Errorf("agui.lock 解析失败：%v", err)
	}
	bad := 0
	installed := map[string]bool{}
	for _, e := range lf.Packages {
		installed[e.Name] = true
	}
	for _, e := range lf.Packages {
		for dep := range e.Dependencies {
			if !installed[dep] {
				fmt.Printf("缺依赖  %-20s -> %s\n", e.Name, dep)
				bad++
			}
		}
	}
	for _, e := range lf.Packages {
		data, err := os.ReadFile(filepath.Join(packagesDir(), filepath.FromSlash(e.Entry)))
		if err != nil {
			fmt.Printf("缺失  %-20s %s\n", e.Name, e.SHA256[:12])
			bad++
			continue
		}
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != e.SHA256 {
			fmt.Printf("变更  %-20s %s -> %s\n", e.Name, e.SHA256[:12], got[:12])
			bad++
		} else {
			fmt.Printf("正常  %-20s %s\n", e.Name, e.SHA256[:12])
		}
	}
	if bad > 0 {
		return fmt.Errorf("%d 个包校验失败", bad)
	}
	fmt.Printf("全部 %d 个包校验通过\n", len(lf.Packages))
	return nil
}

func listPackages() error {
	root := packagesDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("没有已安装的包")
			return nil
		}
		return err
	}
	names := []string{}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			if !seen[e.Name()] {
				names = append(names, e.Name())
				seen[e.Name()] = true
			}
			continue
		}
		if strings.HasSuffix(e.Name(), ".ag") {
			n := strings.TrimSuffix(e.Name(), ".ag")
			if !seen[n] {
				names = append(names, n)
				seen[n] = true
			}
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		fmt.Println("没有已安装的包")
		return nil
	}
	for _, n := range names {
		meta := packageMeta{Name: n}
		if b, err := os.ReadFile(filepath.Join(root, n+".json")); err == nil {
			_ = json.Unmarshal(b, &meta)
		}
		if meta.Version == "" {
			meta.Version = "?"
		}
		deps := ""
		if len(meta.Dependencies) > 0 {
			keys := make([]string, 0, len(meta.Dependencies))
			for d := range meta.Dependencies {
				keys = append(keys, d)
			}
			sort.Strings(keys)
			deps = " 依赖: " + strings.Join(keys, ",")
		}
		fmt.Printf("%-24s %-10s %s%s\n", meta.Name, meta.Version, meta.Source, deps)
	}
	return nil
}

func removePackage(name string, force bool) error {
	clean, err := safePackageName(name)
	if err != nil {
		return err
	}
	metas, _ := installedMetas()
	var blockers []string
	for _, m := range metas {
		if m.Name == clean {
			continue
		}
		if _, ok := m.Dependencies[clean]; ok {
			blockers = append(blockers, m.Name)
		}
	}
	if len(blockers) > 0 && !force {
		sort.Strings(blockers)
		return fmt.Errorf("以下包依赖 %s：%s；使用 --force 强制删除", clean, strings.Join(blockers, ", "))
	}
	root := packagesDir()
	removed := false
	for _, p := range []string{
		filepath.Join(root, clean),
		filepath.Join(root, clean+".ag"),
		filepath.Join(root, clean+".json"),
	} {
		if _, err := os.Stat(p); err == nil {
			if err := os.RemoveAll(p); err != nil {
				return err
			}
			removed = true
		}
	}
	if !removed {
		return fmt.Errorf("没有安装包 %q", clean)
	}
	removeLockEntry(clean)
	fmt.Printf("已删除包 %s\n", clean)
	return nil
}

func removeLockEntry(name string) {
	b, err := os.ReadFile(lockPath())
	if err != nil {
		return
	}
	var lf lockFile
	if json.Unmarshal(b, &lf) != nil {
		return
	}
	out := lf.Packages[:0]
	for _, e := range lf.Packages {
		if e.Name != name {
			out = append(out, e)
		}
	}
	lf.Packages = out
	data, _ := json.MarshalIndent(lf, "", "  ")
	_ = os.WriteFile(lockPath(), append(data, '\n'), 0o644)
}
