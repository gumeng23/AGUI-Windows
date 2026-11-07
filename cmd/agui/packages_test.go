package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallListRemoveLocalPackage(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "mypkg")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := "func add(a, b):\n    return a + b\n"
	if err := os.WriteFile(filepath.Join(src, "mathx.ag"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if err := installPackage(src, "mathx"); err != nil {
		t.Fatal(err)
	}
	got := filepath.Join(root, "packages", "mathx", "mathx.ag")
	if b, err := os.ReadFile(got); err != nil || string(b) != entry {
		t.Fatalf("安装内容不正确: %v %q", err, b)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "mathx.json")); err != nil {
		t.Fatal(err)
	}
	if err := removePackage("mathx"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "mathx")); !os.IsNotExist(err) {
		t.Fatalf("包目录未删除: %v", err)
	}
}
