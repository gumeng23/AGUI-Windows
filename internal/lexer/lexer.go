// Package lexer turns Agui source text into tokens.
//
// Agui is indentation sensitive: a newline ends a statement, and an increase in
// leading whitespace opens a block, just like Python. Newlines inside
// parentheses, brackets or braces are ignored so that long expressions and
// lists can span several lines.
package lexer

import (
	"fmt"
	"strings"
	"unicode"

	"agui/internal/token"
)

// Error is a lexical error with a source position.
type Error struct {
	Line int
	Col  int
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("第 %d 行第 %d 列: %s", e.Line, e.Col, e.Msg)
}

var keywords = map[string]token.Type{
	"if": token.IF, "elif": token.ELIF, "else": token.ELSE,
	"while": token.WHILE, "for": token.FOR, "in": token.IN,
	"func": token.FUNC, "return": token.RETURN,
	"break": token.BREAK, "continue": token.CONTINUE,
	"on": token.ON, "pass": token.PASS,
	"try": token.TRY, "catch": token.CATCH, "finally": token.FINALLY,
	"raise": token.RAISE, "class": token.CLASS,
	"import": token.IMPORT, "lambda": token.LAMBDA,
	"and": token.AND, "or": token.OR, "not": token.NOT,
	"true": token.TRUE, "false": token.FALSE, "null": token.NULL,

	// 中文关键字别名
	"如果":   token.IF,
	"否则如果": token.ELIF,
	"否则":   token.ELSE,
	"当":    token.WHILE,
	"对于":   token.FOR,
	"在":    token.IN,
	"函数":   token.FUNC,
	"返回":   token.RETURN,
	"中断":   token.BREAK,
	"继续":   token.CONTINUE,
	"监听":   token.ON,
	"略过":   token.PASS,
	"尝试":   token.TRY,
	"捕获":   token.CATCH,
	"导入":   token.IMPORT,
	"匿名":   token.LAMBDA,
	"最终":   token.FINALLY,
	"抛出":   token.RAISE,
	"类":    token.CLASS,
	"并且":   token.AND,
	"且":    token.AND,
	"或者":   token.OR,
	"或":    token.OR,
	"非":    token.NOT,
	"真":    token.TRUE,
	"假":    token.FALSE,
	"空":    token.NULL,
}

type lexer struct {
	src          []rune
	i            int
	line         int
	col          int
	depth        int
	atLineStart  bool
	lineHasToken bool
	indents      []int
	toks         []token.Token
}

// Lex scans a complete source file.
func Lex(src string) ([]token.Token, error) {
	src = strings.TrimPrefix(src, "\ufeff")
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")

	l := &lexer{
		src:         []rune(src),
		line:        1,
		col:         1,
		atLineStart: true,
		indents:     []int{0},
	}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.toks, nil
}

func (l *lexer) run() error {
	for l.i < len(l.src) {
		if l.atLineStart {
			if l.depth == 0 {
				if err := l.handleIndent(); err != nil {
					return err
				}
			}
			l.atLineStart = false
		}

		ch := l.src[l.i]
		switch {
		case ch == '\n':
			l.i++
			l.line++
			l.col = 1
			if l.depth == 0 {
				if l.lineHasToken {
					l.emit(token.NEWLINE, "")
					l.lineHasToken = false
				}
				l.atLineStart = true
			}
			continue

		case ch == ' ' || ch == '\t':
			l.i++
			l.col++
			continue

		case ch == '#':
			l.skipToLineEnd()
			continue

		case ch == '/' && l.peek(1) == '/':
			l.skipToLineEnd()
			continue

		case ch == '"' || ch == '\'':
			line, col := l.line, l.col
			s, err := l.scanString()
			if err != nil {
				return err
			}
			l.emitAt(token.STRING, s, line, col)
			continue

		case unicode.IsDigit(ch):
			line, col := l.line, l.col
			lit, err := l.scanNumber()
			if err != nil {
				return err
			}
			l.emitAt(token.NUMBER, lit, line, col)
			continue

		case isIdentStart(ch):
			line, col := l.line, l.col
			name := l.scanIdent()
			if typ, ok := keywords[name]; ok {
				l.emitAt(typ, name, line, col)
			} else {
				l.emitAt(token.IDENT, name, line, col)
			}
			continue
		}

		line, col := l.line, l.col
		switch ch {
		case '+':
			l.op("+=", token.PLUSEQ, token.PLUS, "+", line, col)
		case '-':
			l.op("-=", token.MINUSEQ, token.MINUS, "-", line, col)
		case '*':
			l.op("*=", token.STAREQ, token.STAR, "*", line, col)
		case '/':
			l.op("/=", token.SLASHEQ, token.SLASH, "/", line, col)
		case '%':
			l.emitSingle(token.PERCENT, "%", line, col)
		case '=':
			l.op("==", token.EQ, token.ASSIGN, "=", line, col)
		case '!':
			if l.match("!=") {
				l.emitAt(token.NE, "!=", line, col)
			} else {
				return l.errorAt(line, col, "不认识的字符 '!'（不等于请用 !=，逻辑非请用 not）")
			}
		case '<':
			l.op("<=", token.LE, token.LT, "<", line, col)
		case '>':
			l.op(">=", token.GE, token.GT, ">", line, col)
		case '(':
			l.depth++
			l.emitSingle(token.LPAREN, "(", line, col)
		case ')':
			l.depth--
			if l.depth < 0 {
				return l.errorAt(line, col, "多余的 ')'")
			}
			l.emitSingle(token.RPAREN, ")", line, col)
		case '[':
			l.depth++
			l.emitSingle(token.LBRACKET, "[", line, col)
		case ']':
			l.depth--
			if l.depth < 0 {
				return l.errorAt(line, col, "多余的 ']'")
			}
			l.emitSingle(token.RBRACKET, "]", line, col)
		case '{':
			l.depth++
			l.emitSingle(token.LBRACE, "{", line, col)
		case '}':
			l.depth--
			if l.depth < 0 {
				return l.errorAt(line, col, "多余的 '}'")
			}
			l.emitSingle(token.RBRACE, "}", line, col)
		case ',':
			l.emitSingle(token.COMMA, ",", line, col)
		case ':':
			l.emitSingle(token.COLON, ":", line, col)
		case '.':
			l.emitSingle(token.DOT, ".", line, col)
		case ';':
			l.emitSingle(token.SEMICOLON, ";", line, col)
		default:
			return l.errorAt(line, col, fmt.Sprintf("不认识的字符 %q", ch))
		}
	}

	if l.depth > 0 {
		return l.errorAt(l.line, l.col, "括号没有闭合")
	}
	if l.lineHasToken {
		l.emit(token.NEWLINE, "")
	}
	for len(l.indents) > 1 {
		l.indents = l.indents[:len(l.indents)-1]
		l.emit(token.DEDENT, "")
	}
	l.emit(token.EOF, "")
	return nil
}

// handleIndent processes the leading whitespace of a new logical line.
func (l *lexer) handleIndent() error {
	indent := 0
	for l.i < len(l.src) {
		switch l.src[l.i] {
		case ' ':
			indent++
			l.i++
			l.col++
		case '\t':
			indent += 4
			l.i++
			l.col++
		default:
			goto measured
		}
	}

measured:
	if l.i >= len(l.src) {
		return nil
	}
	ch := l.src[l.i]
	if ch == '\n' || ch == '#' || (ch == '/' && l.peek(1) == '/') {
		// 空行或注释行不影响缩进
		return nil
	}

	top := l.indents[len(l.indents)-1]
	if indent > top {
		l.indents = append(l.indents, indent)
		l.emit(token.INDENT, "")
	} else if indent < top {
		for indent < l.indents[len(l.indents)-1] {
			l.indents = l.indents[:len(l.indents)-1]
			l.emit(token.DEDENT, "")
		}
		if indent != l.indents[len(l.indents)-1] {
			return l.errorAt(l.line, l.col, "缩进不一致，请检查空格数量")
		}
	}
	return nil
}

func (l *lexer) peek(n int) rune {
	if l.i+n >= len(l.src) {
		return 0
	}
	return l.src[l.i+n]
}

func (l *lexer) match(s string) bool {
	rs := []rune(s)
	if l.i+len(rs) > len(l.src) {
		return false
	}
	for k, r := range rs {
		if l.src[l.i+k] != r {
			return false
		}
	}
	l.i += len(rs)
	l.col += len(rs)
	return true
}

// advanceChar consumes exactly one rune of an operator or punctuation mark.
func (l *lexer) advanceChar() {
	l.i++
	l.col++
}

// emitSingle emits a one-character token and consumes it.
func (l *lexer) emitSingle(typ token.Type, lit string, line, col int) {
	l.advanceChar()
	l.emitAt(typ, lit, line, col)
}

// op emits a two-character operator when pair matches, otherwise the
// single-character operator.
func (l *lexer) op(pair string, pairType token.Type, singleType token.Type, lit string, line, col int) {
	if l.match(pair) {
		l.emitAt(pairType, pair, line, col)
		return
	}
	l.emitSingle(singleType, lit, line, col)
}

func (l *lexer) scanIdent() string {
	start := l.i
	for l.i < len(l.src) {
		ch := l.src[l.i]
		if !isIdentPart(ch) {
			break
		}
		l.i++
		l.col++
	}
	return string(l.src[start:l.i])
}

func (l *lexer) scanNumber() (string, error) {
	start := l.i
	if l.src[l.i] == '0' && l.i+1 < len(l.src) && (l.src[l.i+1] == 'x' || l.src[l.i+1] == 'X') {
		l.i += 2
		l.col += 2
		digits := 0
		for l.i < len(l.src) && isHex(l.src[l.i]) {
			l.i++
			l.col++
			digits++
		}
		if digits == 0 {
			return "", l.errorAt(l.line, l.col, "十六进制数字缺少内容")
		}
		return string(l.src[start:l.i]), nil
	}

	for l.i < len(l.src) && unicode.IsDigit(l.src[l.i]) {
		l.i++
		l.col++
	}
	if l.i < len(l.src) && l.src[l.i] == '.' && l.i+1 < len(l.src) && unicode.IsDigit(l.src[l.i+1]) {
		l.i++
		l.col++
		for l.i < len(l.src) && unicode.IsDigit(l.src[l.i]) {
			l.i++
			l.col++
		}
	}
	if l.i < len(l.src) && (l.src[l.i] == 'e' || l.src[l.i] == 'E') {
		save, saveCol := l.i, l.col
		l.i++
		l.col++
		if l.i < len(l.src) && (l.src[l.i] == '+' || l.src[l.i] == '-') {
			l.i++
			l.col++
		}
		if l.i >= len(l.src) || !unicode.IsDigit(l.src[l.i]) {
			l.i, l.col = save, saveCol
		} else {
			for l.i < len(l.src) && unicode.IsDigit(l.src[l.i]) {
				l.i++
				l.col++
			}
		}
	}
	return string(l.src[start:l.i]), nil
}

func (l *lexer) scanString() (string, error) {
	quote := l.src[l.i]
	startLine, startCol := l.line, l.col
	l.i++
	l.col++
	var b strings.Builder
	for {
		if l.i >= len(l.src) {
			return "", l.errorAt(startLine, startCol, "字符串没有结束引号")
		}
		ch := l.src[l.i]
		if ch == quote {
			l.i++
			l.col++
			return b.String(), nil
		}
		if ch == '\n' {
			return "", l.errorAt(startLine, startCol, "字符串不能跨行")
		}
		if ch != '\\' {
			b.WriteRune(ch)
			l.i++
			l.col++
			continue
		}

		// 转义序列
		l.i++
		l.col++
		if l.i >= len(l.src) {
			return "", l.errorAt(startLine, startCol, "字符串没有结束引号")
		}
		esc := l.src[l.i]
		l.i++
		l.col++
		switch esc {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '0':
			b.WriteByte(0)
		case '\\', '"', '\'':
			b.WriteRune(esc)
		case 'x':
			r, err := l.readHex(startLine, startCol, 2)
			if err != nil {
				return "", err
			}
			b.WriteRune(r)
		case 'u':
			if l.i < len(l.src) && l.src[l.i] == '{' {
				l.i++
				l.col++
				n := 0
				val := 0
				for l.i < len(l.src) && l.src[l.i] != '}' {
					d := hexVal(l.src[l.i])
					if d < 0 || n >= 6 {
						return "", l.errorAt(startLine, startCol, "无效的 \\u{...} 转义")
					}
					val = val*16 + d
					n++
					l.i++
					l.col++
				}
				if l.i >= len(l.src) || l.src[l.i] != '}' || n == 0 {
					return "", l.errorAt(startLine, startCol, "无效的 \\u{...} 转义")
				}
				l.i++
				l.col++
				if val > 0x10FFFF {
					return "", l.errorAt(startLine, startCol, "Unicode 码点超出范围")
				}
				b.WriteRune(rune(val))
			} else {
				r, err := l.readHex(startLine, startCol, 4)
				if err != nil {
					return "", err
				}
				b.WriteRune(r)
			}
		default:
			return "", l.errorAt(startLine, startCol, fmt.Sprintf("无效的转义 \\%c", esc))
		}
	}
}

func (l *lexer) readHex(line, col, n int) (rune, error) {
	if l.i+n > len(l.src) {
		return 0, l.errorAt(line, col, "转义序列不完整")
	}
	val := 0
	for k := 0; k < n; k++ {
		d := hexVal(l.src[l.i+k])
		if d < 0 {
			return 0, l.errorAt(line, col, "转义序列包含非十六进制字符")
		}
		val = val*16 + d
	}
	l.i += n
	l.col += n
	return rune(val), nil
}

func (l *lexer) skipToLineEnd() {
	for l.i < len(l.src) && l.src[l.i] != '\n' {
		l.i++
		l.col++
	}
}

func (l *lexer) emit(typ token.Type, lit string) {
	l.emitAt(typ, lit, l.line, l.col)
}

func (l *lexer) emitAt(typ token.Type, lit string, line, col int) {
	l.toks = append(l.toks, token.Token{Type: typ, Lit: lit, Line: line, Col: col})
	if typ != token.NEWLINE && typ != token.INDENT && typ != token.DEDENT {
		l.lineHasToken = true
	}
}

func (l *lexer) errorAt(line, col int, msg string) error {
	return &Error{Line: line, Col: col, Msg: msg}
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isHex(r rune) bool {
	return hexVal(r) >= 0
}

func hexVal(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10
	}
	return -1
}
