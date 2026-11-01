// Package token defines the lexical tokens of the Agui language.
package token

import "fmt"

// Type is the kind of a token.
type Type int

const (
	EOF Type = iota
	NEWLINE
	INDENT
	DEDENT

	IDENT
	NUMBER
	STRING

	// keywords
	IF
	ELIF
	ELSE
	WHILE
	FOR
	IN
	FUNC
	RETURN
	BREAK
	CONTINUE
	ON
	PASS
	TRY
	CATCH
	FINALLY
	RAISE
	IMPORT
	LAMBDA
	CLASS
	AND
	OR
	NOT
	TRUE
	FALSE
	NULL

	// operators
	PLUS
	MINUS
	STAR
	SLASH
	PERCENT

	EQ
	NE
	LT
	LE
	GT
	GE

	ASSIGN
	PLUSEQ
	MINUSEQ
	STAREQ
	SLASHEQ

	LPAREN
	RPAREN
	LBRACKET
	RBRACKET
	LBRACE
	RBRACE
	COMMA
	COLON
	DOT
	SEMICOLON
)

var names = map[Type]string{
	EOF: "文件结尾", NEWLINE: "换行", INDENT: "缩进", DEDENT: "取消缩进",
	IDENT: "标识符", NUMBER: "数字", STRING: "字符串",
	IF: "if", ELIF: "elif", ELSE: "else", WHILE: "while", FOR: "for", IN: "in",
	FUNC: "func", RETURN: "return", BREAK: "break", CONTINUE: "continue",
	ON: "on", PASS: "pass", TRY: "try", CATCH: "catch", FINALLY: "finally",
	RAISE: "raise", IMPORT: "import", LAMBDA: "lambda", CLASS: "class",
	AND: "and", OR: "or", NOT: "not",
	TRUE: "true", FALSE: "false", NULL: "null",
	PLUS: "+", MINUS: "-", STAR: "*", SLASH: "/", PERCENT: "%",
	EQ: "==", NE: "!=", LT: "<", LE: "<=", GT: ">", GE: ">=",
	ASSIGN: "=", PLUSEQ: "+=", MINUSEQ: "-=", STAREQ: "*=", SLASHEQ: "/=",
	LPAREN: "(", RPAREN: ")", LBRACKET: "[", RBRACKET: "]",
	LBRACE: "{", RBRACE: "}", COMMA: ",", COLON: ":", DOT: ".", SEMICOLON: ";",
}

func (t Type) String() string {
	if s, ok := names[t]; ok {
		return s
	}
	return fmt.Sprintf("token(%d)", int(t))
}

// Token is a single lexical unit with its source position (1-based).
type Token struct {
	Type Type
	Lit  string
	Line int
	Col  int
}

func (t Token) String() string {
	if t.Lit != "" {
		return fmt.Sprintf("%s(%q)", t.Type, t.Lit)
	}
	return t.Type.String()
}
