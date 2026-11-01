// Package ast defines the syntax tree of Agui programs.
package ast

import "agui/internal/token"

// Node is any syntax node with a 1-based source line.
type Node interface {
	LineNo() int
}

// Base carries the source line of a node.
type Base struct {
	Line int
}

func (b *Base) LineNo() int { return b.Line }

// Stmt is a statement node.
type Stmt interface {
	Node
}

// Expr is an expression node.
type Expr interface {
	Node
}

// Program is a parsed source file.
type Program struct {
	Stmts []Stmt
}

// ---- statements ----

type ExprStmt struct {
	Base
	X Expr
}

type AssignStmt struct {
	Base
	Target Expr
	Op     token.Type // ASSIGN, PLUSEQ, MINUSEQ, STAREQ, SLASHEQ
	Value  Expr
}

type ElifBranch struct {
	Cond Expr
	Body []Stmt
	Line int
}

type IfStmt struct {
	Base
	Cond  Expr
	Then  []Stmt
	Elifs []ElifBranch
	Else  []Stmt
}

type WhileStmt struct {
	Base
	Cond Expr
	Body []Stmt
}

type ForStmt struct {
	Base
	Name string
	Iter Expr
	Body []Stmt
}

type Param struct {
	Name    string
	Default Expr
	Line    int
}

type FuncStmt struct {
	Base
	Name   string
	Params []Param
	Body   []Stmt
}

type ReturnStmt struct {
	Base
	Value Expr // may be nil
}

type BreakStmt struct{ Base }

type ContinueStmt struct{ Base }

// OnStmt registers an event handler: on widget.event: ...
type OnStmt struct {
	Base
	Target Expr // must be *AttrExpr
	Event  string
	Body   []Stmt
}

// TryStmt runs Body and, on a runtime error, runs Catch with the error message
// bound to CatchName.
type TryStmt struct {
	Base
	Body      []Stmt
	HasCatch  bool
	CatchName string
	Catch     []Stmt
	Finally   []Stmt
}

// RaiseStmt throws a runtime error with a user supplied message.
type RaiseStmt struct {
	Base
	Value Expr
}

// ClassStmt defines a lightweight class. Only methods are allowed in the body.
type ClassStmt struct {
	Base
	Name    string
	Methods []*FuncStmt
}

// ImportStmt loads another .ag source file into the current scope.
type ImportStmt struct {
	Base
	Path Expr
}

type PassStmt struct{ Base }

// ---- expressions ----

type Ident struct {
	Base
	Name string
}

type NumberLit struct {
	Base
	Value float64
}

type StringLit struct {
	Base
	Value string
}

type BoolLit struct {
	Base
	Value bool
}

type NullLit struct{ Base }

type ListLit struct {
	Base
	Elems []Expr
}

type MapLit struct {
	Base
	Keys []Expr
	Vals []Expr
}

type UnaryExpr struct {
	Base
	Op token.Type // MINUS, NOT
	X  Expr
}

type BinaryExpr struct {
	Base
	Op          token.Type
	Left, Right Expr
}

type KwArg struct {
	Name  string
	Value Expr
	Line  int
}

type CallExpr struct {
	Base
	Callee Expr
	Args   []Expr
	Kwargs []KwArg
}

type AttrExpr struct {
	Base
	Obj  Expr
	Name string
}

type IndexExpr struct {
	Base
	Obj   Expr
	Index Expr
}

// SliceExpr is obj[lo:hi:step]; any part may be nil.
type SliceExpr struct {
	Base
	Obj          Expr
	Lo, Hi, Step Expr
}

// LambdaExpr is `lambda x, y: expr` or a `lambda x, y:` block.
type LambdaExpr struct {
	Base
	Params []Param
	Body   []Stmt
}
