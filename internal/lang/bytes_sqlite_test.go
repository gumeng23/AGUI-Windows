package lang_test

import (
	"strings"
	"testing"
)

func TestBytesValue(t *testing.T) {
	src := `
b = bytes("Agui")
b = b + "!"
print(type(b), len(b), b.hex())
print(b[0], b.slice(1, 3).to_string())
b.push(33)
print(b.hex())
print("Agui" in b)
`
	out, _ := runSource(t, src, nil)
	for _, want := range []string{"bytes 5 4167756921", "65 gu", "416775692121", "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q：\n%s", want, out)
		}
	}
}

func TestSQLiteBasics(t *testing.T) {
	src := `
db = null
try:
    db = sqlite_open(":memory:")
catch e:
    print("skip", e)
if db != null:
    db.exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, score REAL)")
    r = db.execute("INSERT INTO users(name, score) VALUES (?, ?)", "Alice", 9.5)
    print("changes", r.changes, "last", r.last_id)
    rows = db.query("SELECT id, name, score FROM users")
    print("rows", len(rows), rows[0].name, rows[0].score)
    print("tables", db.tables)
    db.close()
    print("closed", db.closed)
`
	out, _ := runSource(t, src, nil)
	if strings.Contains(out, "skip") {
		t.Skipf("winsqlite3 unavailable: %s", out)
	}
	for _, want := range []string{"changes 1 last 1", "rows 1 Alice 9.5", "users", "closed true"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q：\n%s", want, out)
		}
	}
}
