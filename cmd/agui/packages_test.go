package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemovePackageRespectsReverseDependency(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	dir := filepath.Join(root, "packages")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dep", "app"} {
		if err := os.WriteFile(filepath.Join(dir, name+".ag"), []byte("func f():\n    return 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	depMeta := `{"name":"dep","version":"1.0.0","source":"test","entry":"dep.ag"}`
	appMeta := `{"name":"app","version":"1.0.0","source":"test","entry":"app.ag","dependencies":{"dep":"^1"}}`
	if err := os.WriteFile(filepath.Join(dir, "dep.json"), []byte(depMeta), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.json"), []byte(appMeta), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removePackage("dep", false); err == nil {
		t.Fatal("删除被依赖的包应当失败")
	}
	if err := removePackage("dep", true); err != nil {
		t.Fatal(err)
	}
}

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
	if err := removePackage("mathx", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "mathx")); !os.IsNotExist(err) {
		t.Fatalf("包目录未删除: %v", err)
	}
}
