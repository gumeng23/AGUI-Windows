package parser

import (
	"testing"

	"agui/internal/ast"
	"agui/internal/lexer"
	"agui/internal/token"
)

func parse(t *testing.T, src string) *ast.Program {
	t.Helper()
	toks, err := lexer.Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, err := Parse(toks)
	if err != nil {
		t.Fatal(err)
	}
	return prog
}

func TestParseFunctionAndCall(t *testing.T) {
	prog := parse(t, "func add(a, b = 2):\n    return a + b\nprint(add(1, b=3))\n")
	if len(prog.Stmts) != 2 {
		t.Fatalf("应有 2 条语句，得到 %d", len(prog.Stmts))
	}
	fn, ok := prog.Stmts[0].(*ast.FuncStmt)
	if !ok {
		t.Fatalf("第一条应为函数定义，得到 %T", prog.Stmts[0])
	}
	if fn.Name != "add" || len(fn.Params) != 2 || fn.Params[1].Default == nil {
		t.Fatalf("函数参数解析错误：%+v", fn)
	}
}

func TestParseIfElifElse(t *testing.T) {
	prog := parse(t, "if a:\n    pass\nelif b:\n    pass\nelse:\n    pass\n")
	if len(prog.Stmts) != 1 {
		t.Fatalf("应有 1 条语句，得到 %d", len(prog.Stmts))
	}
	ifs, ok := prog.Stmts[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("应为 if 语句，得到 %T", prog.Stmts[0])
	}
	if len(ifs.Elifs) != 1 || len(ifs.Else) != 1 {
		t.Fatalf("elif/else 解析错误：%+v", ifs)
	}
}

func TestParseOn(t *testing.T) {
	prog := parse(t, "on btn.click:\n    msgbox(\"hi\")\n")
	on, ok := prog.Stmts[0].(*ast.OnStmt)
	if !ok {
		t.Fatalf("应为 on 语句，得到 %T", prog.Stmts[0])
	}
	if on.Event != "click" {
		t.Fatalf("事件名 = %q", on.Event)
	}
}

func TestParsePrecedence(t *testing.T) {
	prog := parse(t, "x = 1 + 2 * 3\n")
	assign := prog.Stmts[0].(*ast.AssignStmt)
	bin, ok := assign.Value.(*ast.BinaryExpr)
	if !ok || bin.Op != token.PLUS {
		t.Fatalf("顶层应为加法：%#v", assign.Value)
	}
	right, ok := bin.Right.(*ast.BinaryExpr)
	if !ok || right.Op != token.STAR {
		t.Fatalf("右侧应为乘法：%#v", bin.Right)
	}
}

func TestParseErrorIndent(t *testing.T) {
	toks, err := lexer.Lex("if x:\npass\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(toks); err == nil {
		t.Fatal("缺少缩进时应该报错")
	}
}
