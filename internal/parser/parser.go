// Package parser builds an Agui syntax tree from tokens.
package parser

import (
	"fmt"
	"strconv"
	"strings"

	"agui/internal/ast"
	"agui/internal/token"
)

// Error is a syntax error with a source position.
type Error struct {
	Line int
	Col  int
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("第 %d 行第 %d 列: %s", e.Line, e.Col, e.Msg)
}

type parser struct {
	toks []token.Token
	i    int
}

// Parse parses a token stream into a program.
func Parse(toks []token.Token) (*ast.Program, error) {
	p := &parser{toks: toks}
	return p.program()
}

func (p *parser) program() (*ast.Program, error) {
	prog := &ast.Program{}
	p.skipSeparators()
	for !p.check(token.EOF) {
		if p.check(token.INDENT) {
			return nil, p.errorf("这里不应该缩进")
		}
		if p.check(token.DEDENT) {
			return nil, p.errorf("多余的缩进结束")
		}
		s, err := p.stmt()
		if err != nil {
			return nil, err
		}
		prog.Stmts = append(prog.Stmts, s)
		if err := p.finishStmt(isBlockStmt(s)); err != nil {
			return nil, err
		}
	}
	return prog, nil
}

func (p *parser) stmt() (ast.Stmt, error) {
	t := p.cur()
	switch t.Type {
	case token.IF:
		return p.parseIf()
	case token.WHILE:
		return p.parseWhile()
	case token.FOR:
		return p.parseFor()
	case token.FUNC:
		return p.parseFunc()
	case token.ON:
		return p.parseOn()
	case token.TRY:
		return p.parseTry()
	case token.RAISE:
		return p.parseRaise()
	case token.CLASS:
		return p.parseClass()
	case token.IMPORT:
		return p.parseImport()
	case token.RETURN:
		return p.parseReturn()
	case token.BREAK:
		p.advance()
		return &ast.BreakStmt{Base: ast.Base{Line: t.Line}}, nil
	case token.CONTINUE:
		p.advance()
		return &ast.ContinueStmt{Base: ast.Base{Line: t.Line}}, nil
	case token.PASS:
		p.advance()
		return &ast.PassStmt{Base: ast.Base{Line: t.Line}}, nil
	case token.NEWLINE, token.SEMICOLON:
		return nil, p.errorf("这里需要一个语句")
	}

	x, err := p.expr()
	if err != nil {
		return nil, err
	}
	switch p.cur().Type {
	case token.ASSIGN, token.PLUSEQ, token.MINUSEQ, token.STAREQ, token.SLASHEQ:
		op := p.cur().Type
		p.advance()
		val, err := p.expr()
		if err != nil {
			return nil, err
		}
		switch x.(type) {
		case *ast.Ident, *ast.AttrExpr, *ast.IndexExpr:
		default:
			return nil, p.errorAt(t.Line, t.Col, "等号左边必须是变量、属性或列表元素")
		}
		return &ast.AssignStmt{Base: ast.Base{Line: t.Line}, Target: x, Op: op, Value: val}, nil
	}
	return &ast.ExprStmt{Base: ast.Base{Line: t.Line}, X: x}, nil
}

func (p *parser) parseIf() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	cond, err := p.expr()
	if err != nil {
		return nil, err
	}
	if err := p.expect(token.COLON, "':'"); err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	s := &ast.IfStmt{Base: ast.Base{Line: t.Line}, Cond: cond, Then: body}
	for p.check(token.ELIF) {
		line := p.cur().Line
		p.advance()
		c, err := p.expr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(token.COLON, "':'"); err != nil {
			return nil, err
		}
		b, err := p.block()
		if err != nil {
			return nil, err
		}
		s.Elifs = append(s.Elifs, ast.ElifBranch{Cond: c, Body: b, Line: line})
	}
	if p.check(token.ELSE) {
		p.advance()
		if err := p.expect(token.COLON, "':'"); err != nil {
			return nil, err
		}
		b, err := p.block()
		if err != nil {
			return nil, err
		}
		s.Else = b
	}
	return s, nil
}

func (p *parser) parseWhile() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	cond, err := p.expr()
	if err != nil {
		return nil, err
	}
	if err := p.expect(token.COLON, "':'"); err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	return &ast.WhileStmt{Base: ast.Base{Line: t.Line}, Cond: cond, Body: body}, nil
}

func (p *parser) parseFor() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	if !p.check(token.IDENT) {
		return nil, p.errorf("for 后面需要变量名")
	}
	name := p.cur().Lit
	p.advance()
	if err := p.expect(token.IN, "'in'"); err != nil {
		return nil, err
	}
	iter, err := p.expr()
	if err != nil {
		return nil, err
	}
	if err := p.expect(token.COLON, "':'"); err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	return &ast.ForStmt{Base: ast.Base{Line: t.Line}, Name: name, Iter: iter, Body: body}, nil
}

func (p *parser) parseFunc() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	if !p.check(token.IDENT) {
		return nil, p.errorf("func 后面需要函数名")
	}
	name := p.cur().Lit
	p.advance()
	if err := p.expect(token.LPAREN, "'('"); err != nil {
		return nil, err
	}
	var params []ast.Param
	seen := map[string]bool{}
	for !p.check(token.RPAREN) && !p.check(token.EOF) {
		if !p.check(token.IDENT) {
			return nil, p.errorf("函数参数需要变量名")
		}
		pname := p.cur().Lit
		pline := p.cur().Line
		if seen[pname] {
			return nil, p.errorf("参数 %q 重复", pname)
		}
		seen[pname] = true
		p.advance()
		var def ast.Expr
		if p.check(token.ASSIGN) {
			p.advance()
			d, err := p.expr()
			if err != nil {
				return nil, err
			}
			def = d
		}
		params = append(params, ast.Param{Name: pname, Default: def, Line: pline})
		if p.check(token.COMMA) {
			p.advance()
			continue
		}
		break
	}
	if err := p.expect(token.RPAREN, "')'"); err != nil {
		return nil, err
	}
	if err := p.expect(token.COLON, "':'"); err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	return &ast.FuncStmt{Base: ast.Base{Line: t.Line}, Name: name, Params: params, Body: body}, nil
}

func (p *parser) parseOn() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	target, err := p.expr()
	if err != nil {
		return nil, err
	}
	attr, ok := target.(*ast.AttrExpr)
	if !ok {
		return nil, p.errorAt(t.Line, t.Col, "on 后面需要写成 控件.事件，例如 on btn.click:")
	}
	if err := p.expect(token.COLON, "':'"); err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	return &ast.OnStmt{Base: ast.Base{Line: t.Line}, Target: attr.Obj, Event: attr.Name, Body: body}, nil
}

func (p *parser) parseTry() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	if err := p.expect(token.COLON, "':'"); err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	stmt := &ast.TryStmt{Base: ast.Base{Line: t.Line}, Body: body}
	if p.check(token.CATCH) {
		p.advance()
		stmt.HasCatch = true
		stmt.CatchName = "err"
		if p.check(token.IDENT) {
			stmt.CatchName = p.cur().Lit
			p.advance()
		}
		if err := p.expect(token.COLON, "':'"); err != nil {
			return nil, err
		}
		catch, err := p.block()
		if err != nil {
			return nil, err
		}
		stmt.Catch = catch
	}
	if p.check(token.FINALLY) {
		p.advance()
		if err := p.expect(token.COLON, "':'"); err != nil {
			return nil, err
		}
		fin, err := p.block()
		if err != nil {
			return nil, err
		}
		stmt.Finally = fin
	}
	if !stmt.HasCatch && len(stmt.Finally) == 0 {
		return nil, p.errorAt(t.Line, 1, "try 后面需要 catch 或 finally 块")
	}
	return stmt, nil
}

func (p *parser) parseRaise() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	if p.check(token.NEWLINE) || p.check(token.SEMICOLON) || p.check(token.DEDENT) || p.check(token.EOF) {
		return nil, p.errorAt(t.Line, t.Col, "raise 后面需要异常信息")
	}
	val, err := p.expr()
	if err != nil {
		return nil, err
	}
	return &ast.RaiseStmt{Base: ast.Base{Line: t.Line}, Value: val}, nil
}

func (p *parser) parseClass() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	if !p.check(token.IDENT) {
		return nil, p.errorf("class 后面需要类名")
	}
	name := p.cur().Lit
	p.advance()
	if err := p.expect(token.COLON, "':'"); err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	methods := make([]*ast.FuncStmt, 0, len(body))
	for _, s := range body {
		switch v := s.(type) {
		case *ast.FuncStmt:
			methods = append(methods, v)
		case *ast.PassStmt:
			// allow an empty class body
		default:
			return nil, p.errorAt(s.LineNo(), 1, "class 里只能定义函数（func）")
		}
	}
	return &ast.ClassStmt{Base: ast.Base{Line: t.Line}, Name: name, Methods: methods}, nil
}

func (p *parser) parseImport() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	path, err := p.expr()
	if err != nil {
		return nil, err
	}
	return &ast.ImportStmt{Base: ast.Base{Line: t.Line}, Path: path}, nil
}

func (p *parser) parseReturn() (ast.Stmt, error) {
	t := p.cur()
	p.advance()
	if p.check(token.NEWLINE) || p.check(token.SEMICOLON) || p.check(token.DEDENT) || p.check(token.EOF) {
		return &ast.ReturnStmt{Base: ast.Base{Line: t.Line}}, nil
	}
	val, err := p.expr()
	if err != nil {
		return nil, err
	}
	return &ast.ReturnStmt{Base: ast.Base{Line: t.Line}, Value: val}, nil
}

func (p *parser) block() ([]ast.Stmt, error) {
	if !p.check(token.NEWLINE) {
		return nil, p.errorf("冒号后面需要换行并缩进")
	}
	p.advance()
	if !p.check(token.INDENT) {
		return nil, p.errorf("这里需要缩进的代码块")
	}
	p.advance()
	var body []ast.Stmt
	for !p.check(token.DEDENT) && !p.check(token.EOF) {
		s, err := p.stmt()
		if err != nil {
			return nil, err
		}
		body = append(body, s)
		if err := p.finishStmt(isBlockStmt(s)); err != nil {
			return nil, err
		}
	}
	if !p.check(token.DEDENT) {
		return nil, p.errorf("代码块缺少结束（缩进不完整）")
	}
	p.advance()
	return body, nil
}

func (p *parser) finishStmt(block bool) error {
	if block {
		p.skipSeparators()
		return nil
	}
	switch p.cur().Type {
	case token.NEWLINE, token.SEMICOLON:
		p.skipSeparators()
		return nil
	case token.DEDENT, token.EOF:
		return nil
	default:
		return p.errorf("语句结束后需要换行")
	}
}

func isBlockStmt(s ast.Stmt) bool {
	switch n := s.(type) {
	case *ast.IfStmt, *ast.WhileStmt, *ast.ForStmt, *ast.FuncStmt, *ast.OnStmt, *ast.TryStmt, *ast.ClassStmt:
		return true
	case *ast.AssignStmt:
		return exprHasLambda(n.Value)
	case *ast.ExprStmt:
		return exprHasLambda(n.X)
	}
	return false
}

// exprHasLambda reports whether an expression is a block lambda. A lambda that
// owns an indented body consumes the DEDENT, so the enclosing statement has no
// NEWLINE left; treat it like a block statement for separator handling.
func exprHasLambda(e ast.Expr) bool {
	_, ok := e.(*ast.LambdaExpr)
	return ok
}

// ---- expressions ----

func (p *parser) expr() (ast.Expr, error) { return p.parseOr() }

func (p *parser) parseOr() (ast.Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.check(token.OR) {
		op := p.cur()
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Base: ast.Base{Line: op.Line}, Op: token.OR, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (ast.Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.check(token.AND) {
		op := p.cur()
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Base: ast.Base{Line: op.Line}, Op: token.AND, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseNot() (ast.Expr, error) {
	if p.check(token.NOT) {
		t := p.cur()
		p.advance()
		x, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Base: ast.Base{Line: t.Line}, Op: token.NOT, X: x}, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (ast.Expr, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for {
		switch p.cur().Type {
		case token.EQ, token.NE, token.LT, token.LE, token.GT, token.GE, token.IN:
			op := p.cur()
			p.advance()
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			left = &ast.BinaryExpr{Base: ast.Base{Line: op.Line}, Op: op.Type, Left: left, Right: right}
		default:
			return left, nil
		}
	}
}

func (p *parser) parseAdd() (ast.Expr, error) {
	left, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.check(token.PLUS) || p.check(token.MINUS) {
		op := p.cur()
		p.advance()
		right, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Base: ast.Base{Line: op.Line}, Op: op.Type, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseMul() (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.check(token.STAR) || p.check(token.SLASH) || p.check(token.PERCENT) {
		op := p.cur()
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Base: ast.Base{Line: op.Line}, Op: op.Type, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (ast.Expr, error) {
	if p.check(token.MINUS) || p.check(token.PLUS) {
		t := p.cur()
		p.advance()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		if t.Type == token.PLUS {
			return x, nil
		}
		return &ast.UnaryExpr{Base: ast.Base{Line: t.Line}, Op: token.MINUS, X: x}, nil
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() (ast.Expr, error) {
	x, err := p.primary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.cur().Type {
		case token.LPAREN:
			call, err := p.finishCall(x)
			if err != nil {
				return nil, err
			}
			x = call
		case token.LBRACKET:
			line := p.cur().Line
			p.advance()
			var lo ast.Expr
			if !p.check(token.COLON) {
				v, err := p.expr()
				if err != nil {
					return nil, err
				}
				lo = v
			}
			if p.check(token.COLON) {
				p.advance()
				var hi ast.Expr
				if !p.check(token.COLON) && !p.check(token.RBRACKET) {
					v, err := p.expr()
					if err != nil {
						return nil, err
					}
					hi = v
				}
				var step ast.Expr
				if p.check(token.COLON) {
					p.advance()
					if !p.check(token.RBRACKET) {
						v, err := p.expr()
						if err != nil {
							return nil, err
						}
						step = v
					}
				}
				if err := p.expect(token.RBRACKET, "']'"); err != nil {
					return nil, err
				}
				x = &ast.SliceExpr{Base: ast.Base{Line: line}, Obj: x, Lo: lo, Hi: hi, Step: step}
				continue
			}
			if lo == nil {
				return nil, p.errorAt(line, 1, "[] 里需要下标或切片范围")
			}
			if err := p.expect(token.RBRACKET, "']'"); err != nil {
				return nil, err
			}
			x = &ast.IndexExpr{Base: ast.Base{Line: line}, Obj: x, Index: lo}
		case token.DOT:
			line := p.cur().Line
			p.advance()
			if !p.check(token.IDENT) {
				return nil, p.errorf("'.' 后面需要属性名")
			}
			name := p.cur().Lit
			p.advance()
			x = &ast.AttrExpr{Base: ast.Base{Line: line}, Obj: x, Name: name}
		default:
			return x, nil
		}
	}
}

func (p *parser) finishCall(callee ast.Expr) (ast.Expr, error) {
	line := p.cur().Line
	p.advance() // (
	call := &ast.CallExpr{Base: ast.Base{Line: line}, Callee: callee}
	for !p.check(token.RPAREN) && !p.check(token.EOF) {
		if p.check(token.IDENT) && p.peek(1).Type == token.ASSIGN {
			name := p.cur().Lit
			argLine := p.cur().Line
			p.advance()
			p.advance()
			val, err := p.expr()
			if err != nil {
				return nil, err
			}
			call.Kwargs = append(call.Kwargs, ast.KwArg{Name: name, Value: val, Line: argLine})
		} else {
			if len(call.Kwargs) > 0 {
				return nil, p.errorf("位置参数不能放在关键字参数后面")
			}
			arg, err := p.expr()
			if err != nil {
				return nil, err
			}
			call.Args = append(call.Args, arg)
		}
		if p.check(token.COMMA) {
			p.advance()
			continue
		}
		break
	}
	if err := p.expect(token.RPAREN, "')'"); err != nil {
		return nil, err
	}
	return call, nil
}

func (p *parser) primary() (ast.Expr, error) {
	t := p.cur()
	switch t.Type {
	case token.NUMBER:
		p.advance()
		return p.number(t)
	case token.STRING:
		p.advance()
		return &ast.StringLit{Base: ast.Base{Line: t.Line}, Value: t.Lit}, nil
	case token.TRUE:
		p.advance()
		return &ast.BoolLit{Base: ast.Base{Line: t.Line}, Value: true}, nil
	case token.FALSE:
		p.advance()
		return &ast.BoolLit{Base: ast.Base{Line: t.Line}, Value: false}, nil
	case token.NULL:
		p.advance()
		return &ast.NullLit{Base: ast.Base{Line: t.Line}}, nil
	case token.IDENT:
		p.advance()
		return &ast.Ident{Base: ast.Base{Line: t.Line}, Name: t.Lit}, nil
	case token.LPAREN:
		p.advance()
		x, err := p.expr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(token.RPAREN, "')'"); err != nil {
			return nil, err
		}
		return x, nil
	case token.LBRACKET:
		return p.listLit()
	case token.LBRACE:
		return p.mapLit()
	case token.LAMBDA:
		return p.parseLambda()
	default:
		return nil, p.errorf("这里需要一个表达式")
	}
}

func (p *parser) parseLambda() (ast.Expr, error) {
	t := p.cur()
	p.advance()
	parenthesized := false
	if p.check(token.LPAREN) {
		parenthesized = true
		p.advance()
	}
	var params []ast.Param
	seen := map[string]bool{}
	for {
		if parenthesized && p.check(token.RPAREN) {
			break
		}
		if !parenthesized && p.check(token.COLON) {
			break
		}
		if p.check(token.EOF) {
			return nil, p.errorf("lambda 参数列表没有结束")
		}
		if !p.check(token.IDENT) {
			return nil, p.errorf("lambda 参数需要变量名")
		}
		name := p.cur().Lit
		line := p.cur().Line
		if seen[name] {
			return nil, p.errorf("参数 %q 重复", name)
		}
		seen[name] = true
		p.advance()
		var def ast.Expr
		if p.check(token.ASSIGN) {
			p.advance()
			d, err := p.expr()
			if err != nil {
				return nil, err
			}
			def = d
		}
		params = append(params, ast.Param{Name: name, Default: def, Line: line})
		if p.check(token.COMMA) {
			p.advance()
			continue
		}
		break
	}
	if parenthesized {
		if err := p.expect(token.RPAREN, "')'"); err != nil {
			return nil, err
		}
	}
	if err := p.expect(token.COLON, "':'"); err != nil {
		return nil, err
	}
	if p.check(token.NEWLINE) {
		body, err := p.block()
		if err != nil {
			return nil, err
		}
		if len(body) > 0 {
			if es, ok := body[len(body)-1].(*ast.ExprStmt); ok {
				body[len(body)-1] = &ast.ReturnStmt{Base: ast.Base{Line: es.LineNo()}, Value: es.X}
			}
		}
		return &ast.LambdaExpr{Base: ast.Base{Line: t.Line}, Params: params, Body: body}, nil
	}
	body, err := p.expr()
	if err != nil {
		return nil, err
	}
	return &ast.LambdaExpr{
		Base:   ast.Base{Line: t.Line},
		Params: params,
		Body:   []ast.Stmt{&ast.ReturnStmt{Base: ast.Base{Line: t.Line}, Value: body}},
	}, nil
}

func (p *parser) number(t token.Token) (ast.Expr, error) {
	lit := t.Lit
	var (
		f   float64
		err error
	)
	if strings.HasPrefix(lit, "0x") || strings.HasPrefix(lit, "0X") {
		var n int64
		n, err = strconv.ParseInt(lit[2:], 16, 64)
		f = float64(n)
	} else {
		f, err = strconv.ParseFloat(lit, 64)
	}
	if err != nil {
		return nil, p.errorAt(t.Line, t.Col, fmt.Sprintf("无效的数字 %q", lit))
	}
	return &ast.NumberLit{Base: ast.Base{Line: t.Line}, Value: f}, nil
}

func (p *parser) listLit() (ast.Expr, error) {
	t := p.cur()
	p.advance()
	var elems []ast.Expr
	for !p.check(token.RBRACKET) && !p.check(token.EOF) {
		x, err := p.expr()
		if err != nil {
			return nil, err
		}
		elems = append(elems, x)
		if p.check(token.COMMA) {
			p.advance()
			continue
		}
		break
	}
	if err := p.expect(token.RBRACKET, "']'"); err != nil {
		return nil, err
	}
	return &ast.ListLit{Base: ast.Base{Line: t.Line}, Elems: elems}, nil
}

func (p *parser) mapLit() (ast.Expr, error) {
	t := p.cur()
	p.advance()
	m := &ast.MapLit{Base: ast.Base{Line: t.Line}}
	for !p.check(token.RBRACE) && !p.check(token.EOF) {
		k, err := p.expr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(token.COLON, "':'（字典的键和值之间）"); err != nil {
			return nil, err
		}
		v, err := p.expr()
		if err != nil {
			return nil, err
		}
		m.Keys = append(m.Keys, k)
		m.Vals = append(m.Vals, v)
		if p.check(token.COMMA) {
			p.advance()
			continue
		}
		break
	}
	if err := p.expect(token.RBRACE, "'}'"); err != nil {
		return nil, err
	}
	return m, nil
}

// ---- helpers ----

func (p *parser) cur() token.Token {
	if p.i >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}
	return p.toks[p.i]
}

func (p *parser) peek(n int) token.Token {
	if p.i+n >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}
	return p.toks[p.i+n]
}

func (p *parser) advance() token.Token {
	t := p.cur()
	if p.i < len(p.toks)-1 {
		p.i++
	}
	return t
}

func (p *parser) check(t token.Type) bool { return p.cur().Type == t }

func (p *parser) expect(t token.Type, what string) error {
	if !p.check(t) {
		return p.errorf("缺少 %s", what)
	}
	p.advance()
	return nil
}

func (p *parser) skipSeparators() {
	for p.check(token.NEWLINE) || p.check(token.SEMICOLON) {
		p.advance()
	}
}

func (p *parser) errorf(format string, args ...any) error {
	t := p.cur()
	msg := fmt.Sprintf(format, args...)
	if t.Type == token.EOF {
		msg += "（文件结尾）"
	} else if t.Lit != "" {
		msg += fmt.Sprintf("，却遇到 %q", t.Lit)
	} else {
		msg += "，却遇到 " + t.Type.String()
	}
	return &Error{Line: t.Line, Col: t.Col, Msg: msg}
}

func (p *parser) errorAt(line, col int, msg string) error {
	return &Error{Line: line, Col: col, Msg: msg}
}
