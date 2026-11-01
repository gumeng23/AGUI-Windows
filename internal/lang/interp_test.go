package lang_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agui/internal/ast"
	"agui/internal/lang"
	"agui/internal/lexer"
	"agui/internal/parser"
)

func parseSrc(t *testing.T, src string) *ast.Program {
	t.Helper()
	toks, err := lexer.Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, err := parser.Parse(toks)
	if err != nil {
		t.Fatal(err)
	}
	return prog
}

func runSource(t *testing.T, src string, setup func(*lang.Interp)) (string, *lang.Interp) {
	t.Helper()
	i := lang.New()
	var out bytes.Buffer
	i.Out = &out
	i.SetSource("test.ag", src)
	if setup != nil {
		setup(i)
	}
	prog := parseSrc(t, src)
	if err := i.Execute(prog); err != nil {
		t.Fatalf("执行失败：%v\n%s", err, i.FormatError(err))
	}
	return out.String(), i
}

func TestArithmeticAndPrint(t *testing.T) {
	out, _ := runSource(t, `
print(1 + 2 * 3)
print(7 / 2)
print(7 % 2)
print(2 + 3 == 5 and not false)
`, nil)
	want := "7\n3.5\n1\ntrue\n"
	if out != want {
		t.Fatalf("输出 = %q，想要 %q", out, want)
	}
}

func TestStringsListsMaps(t *testing.T) {
	out, _ := runSource(t, `
s = "Agui"
print(s.upper(), s.len())
l = [3, 1, 2]
l.sort()
print(l.join("-"))
l.push(4)
print(l)
d = {"a": 1, "b": 2}
print(d["a"] + len(d))
print(d.keys().join(","))
`, nil)
	want := "AGUI 4\n1-2-3\n[1, 2, 3, 4]\n3\na,b\n"
	if out != want {
		t.Fatalf("输出 = %q，想要 %q", out, want)
	}
}

func TestFunctionsRecursionDefaults(t *testing.T) {
	out, _ := runSource(t, `
func fact(n):
    if n <= 1:
        return 1
    return n * fact(n - 1)

func greet(name, greeting = "你好"):
    return greeting + "，" + name

print(fact(5))
print(greet("世界"))
print(greet("Agui", greeting="Hello"))
`, nil)
	want := "120\n你好，世界\nHello，Agui\n"
	if out != want {
		t.Fatalf("输出 = %q，想要 %q", out, want)
	}
}

func TestControlFlowChinese(t *testing.T) {
	out, _ := runSource(t, `
total = 0
对于 i 在 range(10):
    如果 i == 8:
        中断
    如果 i % 2 == 0:
        继续
    total += i
print(total)

当 total > 0:
    total -= 10
print(total)
`, nil)
	want := "16\n-4\n"
	if out != want {
		t.Fatalf("输出 = %q，想要 %q", out, want)
	}
}

func TestClosure(t *testing.T) {
	out, _ := runSource(t, `
func counter():
    n = 0
    func inc():
        n += 1
        return n
    return inc

c = counter()
print(c(), c(), c())
`, nil)
	if out != "1 2 3\n" {
		t.Fatalf("输出 = %q", out)
	}
}

func TestMapFilterMethods(t *testing.T) {
	out, _ := runSource(t, `
func double(x):
    return x * 2

func even(x):
    return x % 2 == 0

print([1, 2, 3].map(double))
print([1, 2, 3, 4].filter(even))
print(format("你好，{name}！你今年 {age} 岁。", name="小明", age=12))
`, nil)
	want := "[2, 4, 6]\n[2, 4]\n你好，小明！你今年 12 岁。\n"
	if out != want {
		t.Fatalf("输出 = %q，想要 %q", out, want)
	}
}

// ---- fake widget for event tests ----

type fakeWidget struct {
	kind     string
	props    map[string]lang.Value
	handlers map[string][]func() error
}

func newFake(kind string) *fakeWidget {
	return &fakeWidget{
		kind:     kind,
		props:    map[string]lang.Value{"text": ""},
		handlers: map[string][]func() error{},
	}
}

func (f *fakeWidget) Kind() string { return f.kind }

func (f *fakeWidget) GetProp(name string) (lang.Value, bool) {
	v, ok := f.props[name]
	return v, ok
}

func (f *fakeWidget) SetProp(name string, v lang.Value) error {
	f.props[name] = v
	return nil
}

func (f *fakeWidget) CallMethod(string, []lang.Value, map[string]lang.Value) (lang.Value, bool, error) {
	return nil, false, nil
}

func (f *fakeWidget) Listen(event string, fn func() error) error {
	f.handlers[event] = append(f.handlers[event], fn)
	return nil
}

func (f *fakeWidget) fire(event string) error {
	for _, fn := range f.handlers[event] {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func TestOnHandler(t *testing.T) {
	w := newFake("button")
	out, it := runSource(t, `
count = 0
on w.click:
    count += 1
    w.text = "点击" + str(count)
print("ready")
`, func(i *lang.Interp) {
		i.Define("w", w)
	})
	if out != "ready\n" {
		t.Fatalf("初始化输出 = %q", out)
	}
	if err := w.fire("click"); err != nil {
		t.Fatalf("handler 错误：%v", err)
	}
	if err := w.fire("click"); err != nil {
		t.Fatalf("handler 错误：%v", err)
	}
	if got, _ := w.props["text"]; got != "点击2" {
		t.Fatalf("控件文本 = %v", got)
	}
	// 顶层变量应被事件处理器更新。
	v, ok := it.Globals.Get("count")
	if !ok || v != float64(2) {
		t.Fatalf("count = %v", v)
	}
}

func TestHandlerError(t *testing.T) {
	w := newFake("button")
	_, it := runSource(t, `
on w.click:
    x = 1 / 0
`, func(i *lang.Interp) {
		i.Define("w", w)
	})
	err := w.fire("click")
	if err == nil {
		t.Fatal("handler 应该返回错误")
	}
	text := it.FormatError(err)
	if !strings.Contains(text, "不能除以零") || !strings.Contains(text, "第 3 行") {
		t.Fatalf("错误信息不完整：\n%s", text)
	}
}

func TestUndefinedVariable(t *testing.T) {
	i := lang.New()
	i.SetSource("bad.ag", "print(nmae)\n")
	prog := parseSrc(t, "print(nmae)\n")
	err := i.Execute(prog)
	if err == nil || !strings.Contains(err.Error(), "未定义的变量") {
		t.Fatalf("应报未定义变量，得到 %v", err)
	}
}

func TestRuntimeTypeError(t *testing.T) {
	i := lang.New()
	i.SetSource("bad.ag", `print("a" - 1)`+"\n")
	prog := parseSrc(t, `print("a" - 1)`+"\n")
	err := i.Execute(prog)
	if err == nil || !strings.Contains(err.Error(), "需要两个数字") {
		t.Fatalf("应报类型错误，得到 %v", err)
	}
}

func TestSlicesAndLambda(t *testing.T) {
	out, _ := runSource(t, `
l = [1, 2, 3, 4, 5]
print(l[1:3])
print(l[:2])
print(l[3:])
print(l[-2:])
print(l[:])
print("abcdef"[1:4])
double = lambda x: x * 2
add = lambda x, y=10: x + y
print(double(21), add(5), add(5, 1))
`, nil)
	want := "[2, 3]\n[1, 2]\n[4, 5]\n[4, 5]\n[1, 2, 3, 4, 5]\nbcd\n42 15 6\n"
	if out != want {
		t.Fatalf("输出 = %q，想要 %q", out, want)
	}
}

func TestTryCatch(t *testing.T) {
	out, _ := runSource(t, `
try:
    print(1 / 0)
catch err:
    print("caught", err.contains("除以零"))
try:
    pass
catch:
    print("no")
print("after")
`, nil)
	if !strings.Contains(out, "caught true") || !strings.Contains(out, "after") {
		t.Fatalf("输出 = %q", out)
	}
}

func TestImportRelative(t *testing.T) {
	dir := t.TempDir()
	lib := "func square(x):\n    return x * x\nname = \"lib\"\n"
	if err := os.WriteFile(filepath.Join(dir, "mylib.ag"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "import \"mylib\"\nprint(square(7), name)\n"
	i := lang.New()
	var out bytes.Buffer
	i.Out = &out
	i.SetSource(filepath.Join(dir, "main.ag"), src)
	if err := i.Execute(parseSrc(t, src)); err != nil {
		t.Fatalf("执行失败：%v\n%s", err, i.FormatError(err))
	}
	if got := out.String(); got != "49 lib\n" {
		t.Fatalf("输出 = %q", got)
	}
}

func TestClassFinallyRaiseAndStep(t *testing.T) {
	out, _ := runSource(t, `
class Counter:
    func init(self, n):
        self.n = n

    func add(self, x):
        self.n += x
        return self.n

c = Counter(10)
print(c.add(5), c.n)

try:
    raise "boom"
catch e:
    print("caught", e.contains("boom"))
finally:
    print("finally1")

try:
    pass
finally:
    print("finally2")

f = lambda x:
    y = x + 1
    y * 2
print("lambda", f(3))
print([0, 1, 2, 3, 4][::-1], [0, 1, 2, 3, 4][1:4:2], "abcdef"[::2])
`, nil)
	want := "15 15\ncaught true\nfinally1\nfinally2\nlambda 8\n[4, 3, 2, 1, 0] [1, 3] ace\n"
	if out != want {
		t.Fatalf("输出 = %q，想要 %q", out, want)
	}
}

func TestStdlibJSONBase64(t *testing.T) {
	out, _ := runSource(t, `
v = {"name": "Agui", "n": 3, "ok": true, "items": [1, 2]}
s = json_encode(v)
print(s)
back = json_decode(s)
print(back.name, back.n, back.items[1])
print(base64_encode("hello"), base64_decode(base64_encode("hello")))
print(base64_encode("a?b", urlsafe=true))
`, nil)
	for _, want := range []string{`"name":"Agui"`, "Agui 3 2", "aGVsbG8= hello", "YT9i"} {
		if !strings.Contains(out, want) {
			t.Fatalf("输出缺少 %q：%q", want, out)
		}
	}
}

func TestStdlibHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 32)
		n, _ := r.Body.Read(body)
		w.Header().Set("X-Test", "yes")
		fmt.Fprintf(w, "hello %s %s", r.Method, body[:n])
	}))
	defer srv.Close()
	src := fmt.Sprintf(`
r = http_get(%q)
print(r.status, r.ok, r.body, r.headers["X-Test"])
p = http_post(%q, "abc")
print(p.status, p.body)
`, srv.URL, srv.URL)
	out, _ := runSource(t, src, nil)
	if !strings.Contains(out, "200 true hello GET  yes") || !strings.Contains(out, "200 hello POST abc") {
		t.Fatalf("输出 = %q", out)
	}
}
