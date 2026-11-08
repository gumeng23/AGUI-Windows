package main

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
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultRegistryURL = "https://api.github.com/repos/gumeng23/AGUI-registry/contents/index.json"

type registryIndex struct {
	Version  int                        `json:"version"`
	Packages map[string]registryPackage `json:"packages"`
}

type registryPackage struct {
	Description string                     `json:"description,omitempty"`
	Latest      string                     `json:"latest,omitempty"`
	Versions    map[string]registryVersion `json:"versions"`
}

type registryVersion struct {
	URL          string            `json:"url"`
	SHA256       string            `json:"sha256"`
	Entry        string            `json:"entry,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

type packageManifest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Entry        string            `json:"entry"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Agui         string            `json:"agui,omitempty"`
}

func isRegistryName(s string) bool {
	if s == "" || strings.ContainsAny(s, `/\:`) || strings.Contains(s, "..") {
		return false
	}
	for i, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		if i == 0 && r == '@' {
			continue
		}
		return false
	}
	return true
}

func splitPackageRef(ref string) (name, constraint string) {
	name = strings.TrimSpace(ref)
	constraint = "latest"
	if i := strings.LastIndex(name, "@"); i > 0 {
		constraint = name[i+1:]
		name = name[:i]
	}
	if constraint == "" {
		constraint = "latest"
	}
	return name, constraint
}

// semver is intentionally small: major.minor.patch with optional prerelease.
type semver struct {
	major, minor, patch int
	pre                 string
	ok                  bool
}

func parseSemver(s string) semver {
	s = strings.TrimSpace(strings.TrimPrefix(s, "v"))
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	pre := ""
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semver{}
	}
	nums := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}
		}
		nums[i] = n
	}
	return semver{major: nums[0], minor: nums[1], patch: nums[2], pre: pre, ok: true}
}

func compareSemver(a, b semver) int {
	if a.major != b.major {
		return compareInt(a.major, b.major)
	}
	if a.minor != b.minor {
		return compareInt(a.minor, b.minor)
	}
	if a.patch != b.patch {
		return compareInt(a.patch, b.patch)
	}
	if a.pre == b.pre {
		return 0
	}
	if a.pre == "" {
		return 1
	}
	if b.pre == "" {
		return -1
	}
	return strings.Compare(a.pre, b.pre)
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func versionSatisfies(version, constraint string) bool {
	v := parseSemver(version)
	if !v.ok {
		return false
	}
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" || constraint == "latest" {
		return true
	}
	if strings.HasPrefix(constraint, "^") {
		base := parseSemver(constraint[1:])
		if !base.ok || v.major != base.major {
			return false
		}
		if base.major == 0 && v.minor != base.minor {
			return false
		}
		return compareSemver(v, base) >= 0
	}
	if strings.HasPrefix(constraint, "~") {
		base := parseSemver(constraint[1:])
		if !base.ok || v.major != base.major || v.minor != base.minor {
			return false
		}
		return compareSemver(v, base) >= 0
	}
	if strings.HasPrefix(constraint, ">=") {
		base := parseSemver(constraint[2:])
		return base.ok && compareSemver(v, base) >= 0
	}
	base := parseSemver(constraint)
	return base.ok && compareSemver(v, base) == 0
}

func pickVersion(pkg registryPackage, constraint string) (string, registryVersion, error) {
	if len(pkg.Versions) == 0 {
		return "", registryVersion{}, fmt.Errorf("包没有可用版本")
	}
	if constraint == "" || constraint == "latest" {
		if pkg.Latest != "" {
			v, ok := pkg.Versions[pkg.Latest]
			if !ok {
				return "", registryVersion{}, fmt.Errorf("latest 版本 %q 不在 versions 中", pkg.Latest)
			}
			return pkg.Latest, v, nil
		}
	}
	names := make([]string, 0, len(pkg.Versions))
	for name := range pkg.Versions {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := parseSemver(names[i]), parseSemver(names[j])
		if a.ok && b.ok {
			return compareSemver(a, b) > 0
		}
		return names[i] > names[j]
	})
	for _, name := range names {
		if versionSatisfies(name, constraint) {
			return name, pkg.Versions[name], nil
		}
	}
	return "", registryVersion{}, fmt.Errorf("没有满足 %q 的版本", constraint)
}

// githubAPIURL converts raw.githubusercontent.com URLs to the GitHub Contents
// API. The raw host is blocked by some networks while api.github.com remains
// reachable.
func githubAPIURL(raw string) (string, bool) {
	const prefix = "https://raw.githubusercontent.com/"
	if !strings.HasPrefix(raw, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(raw, prefix)
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return "", false
	}
	owner := rest[:slash]
	rest = rest[slash+1:]
	slash = strings.IndexByte(rest, '/')
	if slash <= 0 {
		return "", false
	}
	repo := rest[:slash]
	rest = rest[slash+1:]
	// raw URLs are /owner/repo/branch/path; the Contents API omits the branch.
	slash = strings.IndexByte(rest, '/')
	if slash <= 0 {
		return "", false
	}
	path := rest[slash+1:]
	return "https://api.github.com/repos/" + owner + "/" + repo + "/contents/" + path, true
}

func registryGET(url string, limit int64) ([]byte, error) {
	target, api := url, false
	if u, ok := githubAPIURL(url); ok {
		target, api = u, true
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		return nil, fmt.Errorf("URL 必须是 http(s)")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "agui/"+version)
	if api || strings.Contains(target, "api.github.com") {
		req.Header.Set("Accept", "application/vnd.github.raw")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("内容超过 %d 字节限制", limit)
	}
	return data, nil
}

func fetchRegistry(url string) (*registryIndex, error) {
	if strings.TrimSpace(url) == "" {
		url = defaultRegistryURL
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("registry 必须是 http(s) URL")
	}
	data, err := registryGET(url, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("读取 registry 失败：%w", err)
	}
	var idx registryIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("registry JSON 无效：%w", err)
	}
	if idx.Version != 1 {
		return nil, fmt.Errorf("不支持的 registry 版本 %d", idx.Version)
	}
	if idx.Packages == nil {
		idx.Packages = map[string]registryPackage{}
	}
	return &idx, nil
}

func installPackageWith(src, nameArg, registryURL string, force bool) error {
	if strings.TrimSpace(src) == "" {
		return fmt.Errorf("需要包名、URL 或本地目录/文件")
	}
	if _, err := os.Stat(src); err != nil && isRegistryName(src) {
		return installFromRegistry(src, registryURL, force, map[string]bool{})
	}
	return installPackage(src, nameArg)
}

func installFromRegistry(ref, registryURL string, force bool, stack map[string]bool) error {
	name, constraint := splitPackageRef(ref)
	if !isRegistryName(name) {
		return fmt.Errorf("包名不合法：%q", name)
	}
	if stack[name] {
		return fmt.Errorf("检测到循环依赖：%s", name)
	}
	stack[name] = true
	defer delete(stack, name)

	idx, err := fetchRegistry(registryURL)
	if err != nil {
		return err
	}
	pkg, ok := idx.Packages[name]
	if !ok {
		return fmt.Errorf("registry 中没有包 %q", name)
	}
	ver, meta, err := pickVersion(pkg, constraint)
	if err != nil {
		return fmt.Errorf("%s：%w", name, err)
	}
	if meta.URL == "" {
		return fmt.Errorf("%s@%s 缺少 url", name, ver)
	}
	if meta.SHA256 == "" {
		return fmt.Errorf("%s@%s 缺少 sha256", name, ver)
	}
	if meta.Entry == "" {
		meta.Entry = name + ".ag"
	}

	if !force {
		if old, err := readInstalledVersion(name); err == nil && old == ver {
			fmt.Printf("已安装 %s@%s，跳过\n", name, ver)
			return nil
		}
	}

	for dep, depConstraint := range meta.Dependencies {
		if err := installFromRegistry(dep+"@"+depConstraint, registryURL, force, stack); err != nil {
			return fmt.Errorf("依赖 %s：%w", dep, err)
		}
	}

	data, err := downloadFile(meta.URL, 64<<20)
	if err != nil {
		return fmt.Errorf("下载 %s@%s：%w", name, ver, err)
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), meta.SHA256) {
		return fmt.Errorf("%s@%s SHA-256 校验失败", name, ver)
	}

	root := packagesDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(root, "."+name+".tmp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := extractPackageZip(data, tmp); err != nil {
		return err
	}
	manifest, err := readPackageManifest(tmp)
	if err != nil {
		return err
	}
	if manifest.Name != "" && manifest.Name != name {
		return fmt.Errorf("manifest 包名 %q 与 %q 不一致", manifest.Name, name)
	}
	if manifest.Version != "" && manifest.Version != ver {
		return fmt.Errorf("manifest 版本 %q 与 %q 不一致", manifest.Version, ver)
	}
	entry := meta.Entry
	if manifest.Entry != "" {
		entry = manifest.Entry
	}
	if entry == "" {
		entry = name + ".ag"
	}
	if _, err := os.Stat(filepath.Join(tmp, filepath.FromSlash(entry))); err != nil {
		return fmt.Errorf("包入口不存在：%s", entry)
	}
	dst := filepath.Join(root, name)
	if force {
		_ = os.RemoveAll(dst)
	} else if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("包 %q 已存在，请先 remove 或使用 --force", name)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	deps := meta.Dependencies
	if len(manifest.Dependencies) > 0 {
		deps = manifest.Dependencies
	}
	pm := packageMeta{Name: name, Version: ver, Source: meta.URL, Entry: path.Join(name, entry), Dependencies: deps}
	if err := writePackageMeta(pm); err != nil {
		return err
	}
	if err := addLockEntry(pm); err != nil {
		return err
	}
	fmt.Printf("已安装 %s@%s（入口 %s）\n", name, ver, pm.Entry)
	return nil
}

func installedMetas() ([]packageMeta, error) {
	root := packagesDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var metas []packageMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			continue
		}
		var m packageMeta
		if json.Unmarshal(data, &m) != nil || m.Name == "" {
			continue
		}
		metas = append(metas, m)
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })
	return metas, nil
}

func readPackageMeta(name string) (packageMeta, error) {
	data, err := os.ReadFile(filepath.Join(packagesDir(), name+".json"))
	if err != nil {
		return packageMeta{}, err
	}
	var m packageMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return packageMeta{}, err
	}
	return m, nil
}

func searchPackages(keyword, registryURL string) error {
	idx, err := fetchRegistry(registryURL)
	if err != nil {
		return err
	}
	key := strings.ToLower(strings.TrimSpace(keyword))
	names := make([]string, 0, len(idx.Packages))
	for name := range idx.Packages {
		names = append(names, name)
	}
	sort.Strings(names)
	found := 0
	for _, name := range names {
		pkg := idx.Packages[name]
		text := strings.ToLower(name + " " + pkg.Description)
		if key != "" && !strings.Contains(text, key) {
			continue
		}
		latest := pkg.Latest
		if latest == "" {
			latest = highestVersion(pkg)
		}
		fmt.Printf("%-24s %-10s %s\n", name, latest, pkg.Description)
		found++
	}
	if found == 0 {
		fmt.Printf("没有匹配 %q 的包\n", keyword)
	}
	return nil
}

func highestVersion(pkg registryPackage) string {
	best := ""
	for v := range pkg.Versions {
		if best == "" || compareSemver(parseSemver(v), parseSemver(best)) > 0 {
			best = v
		}
	}
	return best
}

func outdatedPackages(registryURL string) error {
	idx, err := fetchRegistry(registryURL)
	if err != nil {
		return err
	}
	metas, err := installedMetas()
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		fmt.Println("没有已安装的包")
		return nil
	}
	found := 0
	for _, m := range metas {
		pkg, ok := idx.Packages[m.Name]
		if !ok {
			fmt.Printf("%-24s %-10s registry 中不存在\n", m.Name, m.Version)
			continue
		}
		latest := pkg.Latest
		if latest == "" {
			latest = highestVersion(pkg)
		}
		if compareSemver(parseSemver(m.Version), parseSemver(latest)) < 0 {
			fmt.Printf("%-24s %-10s -> %s\n", m.Name, m.Version, latest)
			found++
		}
	}
	if found == 0 {
		fmt.Println("所有包都是最新版本")
	}
	return nil
}

func upgradePackages(refs []string, registryURL string, force bool) error {
	metas, err := installedMetas()
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		fmt.Println("没有已安装的包")
		return nil
	}
	byName := map[string]packageMeta{}
	for _, m := range metas {
		byName[m.Name] = m
	}
	if len(refs) == 0 {
		for _, m := range metas {
			refs = append(refs, m.Name)
		}
	}
	for _, ref := range refs {
		name, _ := splitPackageRef(ref)
		old, ok := byName[name]
		if !ok {
			return fmt.Errorf("没有安装包 %q", name)
		}
		idx, err := fetchRegistry(registryURL)
		if err != nil {
			return err
		}
		pkg, ok := idx.Packages[name]
		if !ok {
			return fmt.Errorf("registry 中没有包 %q", name)
		}
		latest := pkg.Latest
		if latest == "" {
			latest = highestVersion(pkg)
		}
		if !force && compareSemver(parseSemver(old.Version), parseSemver(latest)) >= 0 {
			fmt.Printf("%s@%s 已是最新\n", name, old.Version)
			continue
		}
		if err := installFromRegistry(name+"@"+latest, registryURL, true, map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func readInstalledVersion(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(packagesDir(), name+".json"))
	if err != nil {
		return "", err
	}
	var m packageMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return "", err
	}
	return m.Version, nil
}

func writePackageMeta(m packageMeta) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(packagesDir(), m.Name+".json"), append(data, '\n'), 0o644)
}

func readPackageManifest(dir string) (packageManifest, error) {
	var m packageManifest
	data, err := os.ReadFile(filepath.Join(dir, "agui-package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("agui-package.json 无效：%w", err)
	}
	return m, nil
}

func extractPackageZip(data []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("zip 无效：%w", err)
	}
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		clean := path.Clean(name)
		if clean == "." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
			return fmt.Errorf("zip 包含非法路径：%q", f.Name)
		}
		target := filepath.Join(dir, filepath.FromSlash(clean))
		rel, err := filepath.Rel(dir, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("zip 包含非法路径：%q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if f.UncompressedSize64 > 64<<20 {
			return fmt.Errorf("zip 条目过大：%s", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(rc, 64<<20))
		out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}
