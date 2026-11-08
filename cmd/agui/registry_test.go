package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makePackageZip(t *testing.T, name, version, entry string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	manifest := packageManifest{Name: name, Version: version, Entry: entry}
	mb, _ := json.Marshal(manifest)
	for _, f := range []struct {
		name string
		data []byte
	}{
		{"agui-package.json", mb},
		{entry, []byte("func hello():\n    return \"" + version + "\"\n")},
	} {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(f.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInstallFromRegistry(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	zipData := makePackageZip(t, "mathx", "0.2.0", "mathx.ag")
	sum := sha256.Sum256(zipData)
	pkgURL := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mathx-0.2.0.zip" {
			_, _ = w.Write(zipData)
			return
		}
		index := registryIndex{
			Version: 1,
			Packages: map[string]registryPackage{
				"mathx": {
					Latest: "0.2.0",
					Versions: map[string]registryVersion{
						"0.1.0": {URL: "http://invalid", SHA256: "00", Entry: "mathx.ag"},
						"0.2.0": {URL: pkgURL, SHA256: hex.EncodeToString(sum[:]), Entry: "mathx.ag"},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(index)
	}))
	defer srv.Close()
	pkgURL = srv.URL + "/mathx-0.2.0.zip"

	if err := installFromRegistry("mathx", srv.URL+"/index.json", false, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(root, "packages", "mathx", "mathx.ag")
	if _, err := os.Stat(entry); err != nil {
		t.Fatal(err)
	}
	meta, err := readInstalledVersion("mathx")
	if err != nil || meta != "0.2.0" {
		t.Fatalf("版本 = %q, err=%v", meta, err)
	}
	if _, err := os.Stat(filepath.Join(root, "agui.lock")); err != nil {
		t.Fatal(err)
	}
}

func TestOutdatedAndUpgrade(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	oldZip := makePackageZip(t, "mathx", "0.1.0", "mathx.ag")
	newZip := makePackageZip(t, "mathx", "0.2.0", "mathx.ag")
	oldSum := sha256.Sum256(oldZip)
	newSum := sha256.Sum256(newZip)
	oldURL := ""
	newURL := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mathx-0.1.0.zip":
			_, _ = w.Write(oldZip)
			return
		case "/mathx-0.2.0.zip":
			_, _ = w.Write(newZip)
			return
		}
		index := registryIndex{
			Version: 1,
			Packages: map[string]registryPackage{
				"mathx": {
					Latest: "0.2.0",
					Versions: map[string]registryVersion{
						"0.1.0": {URL: oldURL, SHA256: hex.EncodeToString(oldSum[:]), Entry: "mathx.ag"},
						"0.2.0": {URL: newURL, SHA256: hex.EncodeToString(newSum[:]), Entry: "mathx.ag"},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(index)
	}))
	defer srv.Close()
	oldURL = srv.URL + "/mathx-0.1.0.zip"
	newURL = srv.URL + "/mathx-0.2.0.zip"

	if err := installFromRegistry("mathx@0.1.0", srv.URL+"/index.json", false, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	if err := outdatedPackages(srv.URL + "/index.json"); err != nil {
		t.Fatal(err)
	}
	if err := upgradePackages(nil, srv.URL+"/index.json", false); err != nil {
		t.Fatal(err)
	}
	got, err := readInstalledVersion("mathx")
	if err != nil || got != "0.2.0" {
		t.Fatalf("升级后版本 = %q, err=%v", got, err)
	}
}

func TestInstallDependencyAndVersionRange(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	depZip := makePackageZip(t, "base", "0.1.5", "base.ag")
	mainZip := makePackageZip(t, "app", "0.3.0", "app.ag")
	depSum := sha256.Sum256(depZip)
	mainSum := sha256.Sum256(mainZip)
	baseURL := ""
	appURL := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/base.zip":
			_, _ = w.Write(depZip)
		case "/app.zip":
			_, _ = w.Write(mainZip)
		default:
			_ = json.NewEncoder(w).Encode(registryIndex{
				Version: 1,
				Packages: map[string]registryPackage{
					"base": {Versions: map[string]registryVersion{
						"0.1.5": {URL: baseURL, SHA256: hex.EncodeToString(depSum[:]), Entry: "base.ag"},
					}},
					"app": {Versions: map[string]registryVersion{
						"0.3.0": {URL: appURL, SHA256: hex.EncodeToString(mainSum[:]), Entry: "app.ag",
							Dependencies: map[string]string{"base": "^0.1.0"}},
					}},
				},
			})
		}
	}))
	defer srv.Close()
	baseURL = srv.URL + "/base.zip"
	appURL = srv.URL + "/app.zip"

	if err := installFromRegistry("app@^0.3.0", srv.URL+"/index.json", false, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "base", "base.ag")); err != nil {
		t.Fatal("依赖未安装:", err)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "app", "app.ag")); err != nil {
		t.Fatal(err)
	}
	// 同版本重装也应补齐缺失依赖。
	if err := os.RemoveAll(filepath.Join(root, "packages", "base")); err != nil {
		t.Fatal(err)
	}
	if err := installFromRegistry("app@^0.3.0", srv.URL+"/index.json", false, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "base", "base.ag")); err != nil {
		t.Fatal("同版本安装未补齐依赖:", err)
	}
}

func TestZipTraversalRejected(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("../evil.ag")
	_, _ = w.Write([]byte("evil"))
	_ = zw.Close()
	if err := extractPackageZip(buf.Bytes(), t.TempDir()); err == nil {
		t.Fatal("应拒绝目录穿越")
	}
}

func TestGitHubAPIURL(t *testing.T) {
	got, ok := githubAPIURL("https://raw.githubusercontent.com/gumeng23/AGUI-registry/main/packages/mathx-0.1.0.zip")
	if !ok {
		t.Fatal("expected raw.githubusercontent.com URL to be converted")
	}
	want := "https://api.github.com/repos/gumeng23/AGUI-registry/contents/packages/mathx-0.1.0.zip"
	if got != want {
		t.Fatalf("githubAPIURL = %q, want %q", got, want)
	}
	if !strings.HasPrefix(got, "https://api.github.com/repos/") {
		t.Fatalf("unexpected API URL: %s", got)
	}
}

func TestVersionSatisfies(t *testing.T) {
	cases := []struct {
		v, c string
		ok   bool
	}{
		{"0.2.0", "^0.2.0", true},
		{"0.3.0", "^0.2.0", false},
		{"1.4.0", "^1.2.0", true},
		{"1.2.9", "~1.2.3", true},
		{"1.3.0", "~1.2.3", false},
		{"2.0.0", ">=1.5.0", true},
		{"1.0.0", "1.0.0", true},
	}
	for _, c := range cases {
		if got := versionSatisfies(c.v, c.c); got != c.ok {
			t.Errorf("versionSatisfies(%q,%q)=%v want %v", c.v, c.c, got, c.ok)
		}
	}
}

func TestInstallChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	zipData := makePackageZip(t, "bad", "0.1.0", "bad.ag")
	url := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad.zip" {
			_, _ = w.Write(zipData)
			return
		}
		fmt.Fprint(w, `{"version":1,"packages":{"bad":{"versions":{"0.1.0":{"url":"`+url+`","sha256":"deadbeef","entry":"bad.ag"}}}}}`)
	}))
	defer srv.Close()
	url = srv.URL + "/bad.zip"
	if err := installFromRegistry("bad", srv.URL+"/index.json", false, map[string]bool{}); err == nil {
		t.Fatal("应拒绝校验失败的包")
	}
}
