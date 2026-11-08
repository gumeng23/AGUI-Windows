package lang

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agui/internal/lexer"
	"agui/internal/parser"
)

func TestImportUsesPackageMetaEntry(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	pkgDir := filepath.Join(root, "packages", "mathx", "lib")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "impl.ag"), []byte("func add(a, b):\n    return a + b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := `{"name":"mathx","version":"1.0.0","entry":"mathx/lib/impl.ag"}`
	if err := os.WriteFile(filepath.Join(root, "packages", "mathx.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "import \"mathx\"\nprint(mathx.add(2, 3))\n"
	toks, err := lexer.Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, err := parser.Parse(toks)
	if err != nil {
		t.Fatal(err)
	}
	i := New()
	var out bytes.Buffer
	i.Out = &out
	i.SourceName = filepath.Join(root, "main.ag")
	i.SetBaseDir(root)
	if err := i.Execute(prog); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "5" {
		t.Fatalf("输出 = %q，期望 5", got)
	}
}
