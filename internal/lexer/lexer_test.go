package lexer

import (
	"testing"

	"agui/internal/token"
)

func typesOf(toks []token.Token) []token.Type {
	out := make([]token.Type, len(toks))
	for i, tok := range toks {
		out[i] = tok.Type
	}
	return out
}

func TestLexAssignment(t *testing.T) {
	toks, err := Lex("x = 1 + 2\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{
		token.IDENT, token.ASSIGN, token.NUMBER, token.PLUS, token.NUMBER,
		token.NEWLINE, token.EOF,
	}
	got := typesOf(toks)
	if len(got) != len(want) {
		t.Fatalf("token 数量 = %d, 想要 %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个 token = %v, 想要 %v", i, got[i], want[i])
		}
	}
}

func TestLexIndent(t *testing.T) {
	toks, err := Lex("if x:\n    y = 1\nz = 2\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{
		token.IF, token.IDENT, token.COLON, token.NEWLINE,
		token.INDENT, token.IDENT, token.ASSIGN, token.NUMBER, token.NEWLINE,
		token.DEDENT, token.IDENT, token.ASSIGN, token.NUMBER, token.NEWLINE,
		token.EOF,
	}
	got := typesOf(toks)
	if len(got) != len(want) {
		t.Fatalf("token 数量 = %d, 想要 %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个 token = %v, 想要 %v（全部：%v）", i, got[i], want[i], got)
		}
	}
}

// 回归测试：所有运算符都必须消费输入，否则词法分析器会无限循环。
func TestLexOperatorsAdvance(t *testing.T) {
	src := "a = 1\nb += 2\nc -= 3\nd *= 4\ne /= 5\nif a <= b and b != 0 or a >= 1:\n    pass\n"
	toks, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(toks) < 20 {
		t.Fatalf("token 太少，可能没有正确扫描：%v", toks)
	}
}

func TestLexChineseKeywords(t *testing.T) {
	toks, err := Lex("如果 真:\n    略过\n")
	if err != nil {
		t.Fatal(err)
	}
	if toks[0].Type != token.IF || toks[1].Type != token.TRUE {
		t.Fatalf("中文关键字解析失败：%v", toks)
	}
}

func TestLexCommentsAndContinuation(t *testing.T) {
	src := "# comment\nitems = [\n    1,  # one\n    2,\n]\n// tail\n"
	toks, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, tok := range toks {
		if tok.Type == token.NUMBER {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("列表里的数字应为 2 个，得到 %d", count)
	}
}

func TestLexStringEscapes(t *testing.T) {
	toks, err := Lex(`print("a\nb\u4e2d\u6587")`)
	if err != nil {
		t.Fatal(err)
	}
	if toks[2].Type != token.STRING || toks[2].Lit != "a\nb中文" {
		t.Fatalf("字符串转义错误：%q", toks[2].Lit)
	}
}
