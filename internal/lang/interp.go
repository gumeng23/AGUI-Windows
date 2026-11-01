package lang

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"agui/internal/ast"
	"agui/internal/lexer"
	"agui/internal/parser"
	"agui/internal/token"
)

// Error is an Agui runtime error with a source line and call stack.
type Error struct {
	Line  int
	Msg   string
	Stack []string
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("第 %d 行: %s", e.Line, e.Msg)
	}
	return e.Msg
}

type returnSignal struct{ value Value }
type breakSignal struct{}
type continueSignal struct{}

// Interp executes Agui syntax trees.
type Interp struct {
	Globals    *Env
	Out        io.Writer
	SourceName string
	Source     string
	Lines      []string

	line      int
	stack     []string
	callDepth int
	maxDepth  int
	imported  map[string]bool
	mu        reentrantMutex

	coop      bool
	task      *Task
	lastYield time.Time
	assets    *AssetStore
	dispatch  func(func())
}

// New creates an interpreter with all core built-ins installed.
func New() *Interp {
	i := &Interp{
		Globals:    NewEnv(nil),
		Out:        os.Stdout,
		SourceName: "<script>",
		maxDepth:   512,
		imported:   map[string]bool{},
		assets:     newAssetStore(),
	}
	registerBuiltins(i)
	registerStdlib(i)
	registerWebSocket(i)
	return i
}

// SetDispatcher installs a host callback used to deliver network events on the
// UI thread. When it is nil, events run in a protected background goroutine.
func (i *Interp) SetDispatcher(fn func(func())) {
	i.dispatch = fn
}

// Dispatch delivers fn through the host dispatcher, or runs it inline with the
// interpreter lock held (blocking until the current script yields).
func (i *Interp) Dispatch(fn func()) {
	if i.dispatch != nil {
		i.dispatch(fn)
		return
	}
	i.Protect(fn)
}

// SetSource records the source text for error messages and sets Source/Lines.
func (i *Interp) SetSource(name, src string) {
	i.SourceName = name
	i.Source = src
	src = strings.TrimPrefix(src, "\ufeff")
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	i.Lines = strings.Split(src, "\n")
}

// Define registers a global value (used by the GUI host).
func (i *Interp) Define(name string, v Value) {
	i.Globals.Define(name, v)
}

// GlobalsSnapshot returns a copy of the global variables for debugging.
func (i *Interp) GlobalsSnapshot() map[string]Value {
	return i.Globals.Snapshot()
}

// Suspend releases the execution lock around a blocking native call (the GUI
// message loop). Resume must be called when the call returns.
func (i *Interp) Suspend() int {
	return i.mu.Suspend()
}

// Resume reacquires the execution lock released by Suspend.
func (i *Interp) Resume(depth int) {
	i.mu.Resume(depth)
}

// RegisterNative registers a built-in function.
func (i *Interp) RegisterNative(name string, fn func(*Interp, []Value, map[string]Value) (Value, error)) {
	i.Globals.Define(name, &Native{Name: name, Fn: fn})
}

// Execute runs a parsed program.
func (i *Interp) Execute(prog *ast.Program) error {
	i.stack = nil
	i.callDepth = 0
	return i.Protect(func() {
		i.execBlock(prog.Stmts, i.Globals)
	})
}

// Protect runs fn and turns Agui panics into errors. It is used for top level
// execution and for every GUI callback, so one bad handler cannot kill the app.
func (i *Interp) Protect(fn func()) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	oldCoop, oldTask := i.coop, i.task
	i.coop, i.task = false, nil
	defer func() { i.coop, i.task = oldCoop, oldTask }()
	return i.protectPanic(fn)
}

// ProtectCooperative runs fn as a background task. The interpreter lock is
// still held, but background statements periodically suspend it so the UI can
// run. Protect never yields, so callbacks and event handlers stay atomic.
func (i *Interp) ProtectCooperative(task *Task, fn func()) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	oldCoop, oldTask := i.coop, i.task
	i.coop, i.task = true, task
	i.lastYield = time.Now()
	defer func() { i.coop, i.task = oldCoop, oldTask }()
	return i.protectPanic(fn)
}

func (i *Interp) protectPanic(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case breakSignal:
				err = &Error{Line: i.line, Msg: "break 只能用在循环里"}
			case continueSignal:
				err = &Error{Line: i.line, Msg: "continue 只能用在循环里"}
			case returnSignal:
				err = &Error{Line: i.line, Msg: "return 只能用在函数里"}
			default:
				if os.Getenv("AGUI_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "agui internal panic: %v\n%s\n", r, debug.Stack())
				}
				err = i.asError(r)
			}
		}
	}()
	fn()
	return nil
}

// TaskContext returns the current background task context, or nil for UI work.
func (i *Interp) TaskContext() context.Context {
	if i.task == nil {
		return nil
	}
	return i.task.Context()
}

// maybeYield lets the UI run during a long background task.
func (i *Interp) maybeYield() {
	if !i.coop || i.task == nil {
		return
	}
	if i.task.Context().Err() != nil {
		i.throw("任务已取消")
	}
	if time.Since(i.lastYield) < 4*time.Millisecond {
		return
	}
	depth := i.mu.Suspend()
	time.Sleep(time.Millisecond)
	i.lastYield = time.Now()
	i.mu.Resume(depth)
	if i.task.Context().Err() != nil {
		i.throw("任务已取消")
	}
}

// Sleep is the cooperative version used by the sleep() built-in.
func (i *Interp) Sleep(d time.Duration) {
	if !i.coop || i.task == nil {
		time.Sleep(d)
		return
	}
	depth := i.mu.Suspend()
	defer i.mu.Resume(depth)
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-i.task.Context().Done():
		i.throw("任务已取消")
	}
}

// Blocking releases the interpreter lock while fn performs blocking I/O.
func (i *Interp) Blocking(fn func() error) error {
	if !i.coop || i.task == nil {
		return fn()
	}
	depth := i.mu.Suspend()
	err := fn()
	i.mu.Resume(depth)
	return err
}

// CallValue calls a function value with no arguments. The GUI uses it for
// timers and on_click shortcuts.
func (i *Interp) CallValue(fn Value) Value {
	return i.call(fn, nil, nil)
}

func (i *Interp) asError(r any) error {
	switch v := r.(type) {
	case *Error:
		return v
	case error:
		return &Error{Line: i.line, Msg: v.Error()}
	case string:
		return &Error{Line: i.line, Msg: v}
	default:
		return &Error{Line: i.line, Msg: fmt.Sprintf("%v", v)}
	}
}

// FormatError renders an error with the offending source line and call stack.
func (i *Interp) FormatError(err error) string {
	var ae *Error
	if !errors.As(err, &ae) {
		return err.Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s 第 %d 行\n\n错误: %s", i.SourceName, ae.Line, ae.Msg)
	if ae.Line >= 1 && ae.Line <= len(i.Lines) {
		b.WriteString("\n\n")
		start := ae.Line - 1
		if start < 1 {
			start = 1
		}
		end := ae.Line + 1
		if end > len(i.Lines) {
			end = len(i.Lines)
		}
		for n := start; n <= end; n++ {
			marker := "   "
			if n == ae.Line {
				marker = " > "
			}
			fmt.Fprintf(&b, "%s%4d | %s\n", marker, n, i.Lines[n-1])
		}
	}
	if len(ae.Stack) > 0 {
		b.WriteString("\n调用栈:\n")
		for idx := len(ae.Stack) - 1; idx >= 0; idx-- {
			b.WriteString("  " + ae.Stack[idx] + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (i *Interp) throw(format string, args ...any) {
	panic(&Error{
		Line:  i.line,
		Msg:   fmt.Sprintf(format, args...),
		Stack: append([]string(nil), i.stack...),
	})
}

// ---- statements ----

func (i *Interp) execBlock(stmts []ast.Stmt, env *Env) {
	for _, s := range stmts {
		i.execStmt(s, env)
	}
}

func (i *Interp) execStmt(s ast.Stmt, env *Env) {
	i.maybeYield()
	i.line = s.LineNo()
	switch n := s.(type) {
	case *ast.ExprStmt:
		i.eval(n.X, env)
	case *ast.AssignStmt:
		i.execAssign(n, env)
	case *ast.IfStmt:
		if Truthy(i.eval(n.Cond, env)) {
			i.execBlock(n.Then, env)
			return
		}
		for _, br := range n.Elifs {
			if Truthy(i.eval(br.Cond, env)) {
				i.execBlock(br.Body, env)
				return
			}
		}
		i.execBlock(n.Else, env)
	case *ast.WhileStmt:
		for Truthy(i.eval(n.Cond, env)) {
			brk, _ := i.runLoopBody(n.Body, env)
			if brk {
				break
			}
		}
	case *ast.ForStmt:
		items := i.iterate(i.eval(n.Iter, env))
		for _, item := range items {
			env.Define(n.Name, item)
			brk, _ := i.runLoopBody(n.Body, env)
			if brk {
				break
			}
		}
	case *ast.FuncStmt:
		fn := &Function{Name: n.Name, Params: n.Params, Body: n.Body, Env: env, Line: n.LineNo()}
		if old, ok := env.Local(n.Name); ok {
			if oldFn, ok := old.(*Function); ok && oldFn.Name == n.Name {
				panic(&Error{Line: n.LineNo(), Msg: fmt.Sprintf("函数 %q 重复定义", n.Name)})
			}
		}
		env.Define(n.Name, fn)
	case *ast.ReturnStmt:
		var v Value
		if n.Value != nil {
			v = i.eval(n.Value, env)
		}
		panic(returnSignal{value: v})
	case *ast.BreakStmt:
		panic(breakSignal{})
	case *ast.ContinueStmt:
		panic(continueSignal{})
	case *ast.OnStmt:
		i.execOn(n, env)
	case *ast.TryStmt:
		i.execTry(n, env)
	case *ast.RaiseStmt:
		i.throw("%s", ToString(i.eval(n.Value, env)))
	case *ast.ClassStmt:
		i.execClass(n, env)
	case *ast.ImportStmt:
		i.execImport(n, env)
	case *ast.PassStmt:
		// nothing
	default:
		i.throw("未知语句 %T", s)
	}
}

func (i *Interp) execTry(n *ast.TryStmt, env *Env) {
	run := func(stmts []ast.Stmt) (pending any) {
		defer func() { pending = recover() }()
		i.execBlock(stmts, env)
		return nil
	}
	pending := run(n.Body)
	if pending != nil {
		if e, ok := pending.(*Error); ok && n.HasCatch {
			name := n.CatchName
			if name == "" {
				name = "err"
			}
			env.Define(name, e.Error())
			pending = run(n.Catch)
		}
	}
	if len(n.Finally) > 0 {
		if fp := run(n.Finally); fp != nil {
			pending = fp
		}
	}
	if pending != nil {
		panic(pending)
	}
}

func (i *Interp) execClass(n *ast.ClassStmt, env *Env) {
	c := &Class{Name: n.Name, Methods: map[string]*Function{}}
	for _, m := range n.Methods {
		if _, dup := c.Methods[m.Name]; dup {
			i.throw("类 %s 中方法 %q 重复定义", n.Name, m.Name)
		}
		c.Methods[m.Name] = &Function{
			Name:   n.Name + "." + m.Name,
			Params: m.Params,
			Body:   m.Body,
			Env:    env,
			Line:   m.LineNo(),
		}
	}
	env.Define(n.Name, c)
}

func (i *Interp) execImport(n *ast.ImportStmt, env *Env) {
	raw := ToString(i.eval(n.Path, env))
	if strings.TrimSpace(raw) == "" {
		i.throw("import 需要一个文件路径")
	}
	name := filepath.ToSlash(raw)
	if filepath.Ext(name) == "" {
		name += ".ag"
	}
	key := name
	var data []byte
	var err error
	if !filepath.IsAbs(raw) && i.HasAsset(name) {
		key = normalizeAssetPath(name)
		data, _ = i.AssetBytes(name)
	} else {
		p := raw
		if !filepath.IsAbs(p) {
			dir := filepath.Dir(i.SourceName)
			if dir == "." {
				dir = i.BaseDir()
			}
			p = filepath.Join(dir, p)
		}
		if filepath.Ext(p) == "" {
			p += ".ag"
		}
		abs, absErr := filepath.Abs(p)
		if absErr != nil {
			i.throw("import %q: %v", raw, absErr)
		}
		key = abs
		err = i.Blocking(func() error {
			var readErr error
			data, readErr = os.ReadFile(abs)
			return readErr
		})
	}
	if err != nil {
		i.throw("无法导入 %q: %v", raw, err)
	}
	if i.imported[key] {
		return
	}
	toks, err := lexer.Lex(string(data))
	if err != nil {
		i.throw("导入 %q 失败: %v", raw, err)
	}
	prog, err := parser.Parse(toks)
	if err != nil {
		i.throw("导入 %q 失败: %v", raw, err)
	}
	i.imported[key] = true
	defer func() {
		if r := recover(); r != nil {
			delete(i.imported, key)
			panic(r)
		}
	}()
	i.execBlock(prog.Stmts, env)
}
func (i *Interp) execOn(n *ast.OnStmt, env *Env) {
	obj := i.eval(n.Target, env)
	w, ok := obj.(Widget)
	if !ok {
		i.throw("on 的对象必须是控件，而不是 %s", TypeName(obj))
	}
	handler := func() error {
		return i.Protect(func() {
			i.execBlock(n.Body, env)
		})
	}
	if err := w.Listen(n.Event, handler); err != nil {
		i.throw("%s", err.Error())
	}
}

func (i *Interp) execAssign(n *ast.AssignStmt, env *Env) {
	switch target := n.Target.(type) {
	case *ast.Ident:
		var val Value
		if n.Op == token.ASSIGN {
			val = i.eval(n.Value, env)
		} else {
			old, ok := env.Get(target.Name)
			if !ok {
				i.throw("变量 %q 还没有定义，不能使用 %s", target.Name, n.Op)
			}
			val = i.binary(compoundOp(n.Op), old, i.eval(n.Value, env))
		}
		env.Assign(target.Name, val)

	case *ast.AttrExpr:
		obj := i.eval(target.Obj, env)
		if inst, ok := obj.(*Instance); ok {
			var old Value
			if n.Op != token.ASSIGN {
				old = i.getAttr(inst, target.Name)
			}
			val := i.eval(n.Value, env)
			if n.Op != token.ASSIGN {
				val = i.binary(compoundOp(n.Op), old, val)
			}
			inst.Fields.Set(target.Name, val)
			return
		}
		w, ok := obj.(Widget)
		if !ok {
			i.throw("只能给控件或对象设置属性，%s 不是控件", TypeName(obj))
		}
		if n.Op == token.ASSIGN {
			val := i.eval(n.Value, env)
			if err := w.SetProp(target.Name, val); err != nil {
				i.throw("%s", err.Error())
			}
			return
		}
		old, ok := w.GetProp(target.Name)
		if !ok {
			i.throw("控件 %s 没有属性 %q", w.Kind(), target.Name)
		}
		val := i.binary(compoundOp(n.Op), old, i.eval(n.Value, env))
		if err := w.SetProp(target.Name, val); err != nil {
			i.throw("%s", err.Error())
		}

	case *ast.IndexExpr:
		obj := i.eval(target.Obj, env)
		key := i.eval(target.Index, env)
		if n.Op == token.ASSIGN {
			i.setIndex(obj, key, i.eval(n.Value, env))
			return
		}
		old := i.index(obj, key)
		i.setIndex(obj, key, i.binary(compoundOp(n.Op), old, i.eval(n.Value, env)))

	default:
		i.throw("等号左边必须是变量、属性或列表元素")
	}
}

func (i *Interp) runLoopBody(body []ast.Stmt, env *Env) (brk, cont bool) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case breakSignal:
				brk = true
			case continueSignal:
				cont = true
			default:
				panic(r)
			}
		}
	}()
	i.execBlock(body, env)
	return false, false
}

func compoundOp(op token.Type) token.Type {
	switch op {
	case token.PLUSEQ:
		return token.PLUS
	case token.MINUSEQ:
		return token.MINUS
	case token.STAREQ:
		return token.STAR
	case token.SLASHEQ:
		return token.SLASH
	}
	return op
}

// ---- expressions ----

func (i *Interp) eval(e ast.Expr, env *Env) Value {
	i.line = e.LineNo()
	switch n := e.(type) {
	case *ast.NumberLit:
		return n.Value
	case *ast.StringLit:
		return n.Value
	case *ast.BoolLit:
		return n.Value
	case *ast.NullLit:
		return nil
	case *ast.Ident:
		v, ok := env.Get(n.Name)
		if !ok {
			i.throw("未定义的变量 %q", n.Name)
		}
		return v
	case *ast.ListLit:
		items := make([]Value, len(n.Elems))
		for idx, el := range n.Elems {
			items[idx] = i.eval(el, env)
		}
		return &List{Items: items}
	case *ast.MapLit:
		m := NewMap()
		for idx := range n.Keys {
			k := i.eval(n.Keys[idx], env)
			ks, ok := keyString(k)
			if !ok {
				i.throw("字典的键必须是字符串或数字")
			}
			m.Set(ks, i.eval(n.Vals[idx], env))
		}
		return m
	case *ast.UnaryExpr:
		return i.unary(n, env)
	case *ast.BinaryExpr:
		if n.Op == token.AND {
			if !Truthy(i.eval(n.Left, env)) {
				return false
			}
			return Truthy(i.eval(n.Right, env))
		}
		if n.Op == token.OR {
			if Truthy(i.eval(n.Left, env)) {
				return true
			}
			return Truthy(i.eval(n.Right, env))
		}
		return i.binary(n.Op, i.eval(n.Left, env), i.eval(n.Right, env))
	case *ast.CallExpr:
		return i.evalCall(n, env)
	case *ast.AttrExpr:
		obj := i.eval(n.Obj, env)
		switch o := obj.(type) {
		case Widget:
			v, ok := o.GetProp(n.Name)
			if !ok {
				i.throw("控件 %s 没有属性 %q", o.Kind(), n.Name)
			}
			return v
		case *Map:
			v, _ := o.Get(n.Name)
			return v
		case *Instance:
			return i.getAttr(o, n.Name)
		case *Task:
			if v, ok := o.GetProp(n.Name); ok {
				return v
			}
			i.throw("任务没有属性 %q", n.Name)
		case *WebSocket:
			if v, ok := o.GetProp(n.Name); ok {
				return v
			}
			i.throw("WebSocket 没有属性 %q", n.Name)
		case *Class:
			if fn, ok := o.Methods[n.Name]; ok {
				return fn
			}
		}
		i.throw("%s 没有属性 %q", TypeName(obj), n.Name)
	case *ast.IndexExpr:
		return i.index(i.eval(n.Obj, env), i.eval(n.Index, env))
	case *ast.SliceExpr:
		var lo, hi, step Value
		if n.Lo != nil {
			lo = i.eval(n.Lo, env)
		}
		if n.Hi != nil {
			hi = i.eval(n.Hi, env)
		}
		if n.Step != nil {
			step = i.eval(n.Step, env)
		}
		return i.slice(i.eval(n.Obj, env), lo, hi, step, n.Lo != nil, n.Hi != nil, n.Step != nil)
	case *ast.LambdaExpr:
		return &Function{
			Name:   "<lambda>",
			Params: n.Params,
			Body:   n.Body,
			Env:    env,
			Line:   n.LineNo(),
		}
	default:
		i.throw("未知表达式 %T", e)
	}
	return nil
}

func (i *Interp) unary(n *ast.UnaryExpr, env *Env) Value {
	v := i.eval(n.X, env)
	switch n.Op {
	case token.NOT:
		return !Truthy(v)
	case token.MINUS:
		f, ok := asNumber(v)
		if !ok {
			i.throw("取负需要数字，而不是 %s", TypeName(v))
		}
		return -f
	}
	i.throw("未知的一元运算符")
	return nil
}

func (i *Interp) evalCall(n *ast.CallExpr, env *Env) Value {
	args := make([]Value, len(n.Args))
	for idx, a := range n.Args {
		args[idx] = i.eval(a, env)
	}
	kwargs := make(map[string]Value, len(n.Kwargs))
	for _, kw := range n.Kwargs {
		if _, dup := kwargs[kw.Name]; dup {
			i.throw("参数 %q 重复传入", kw.Name)
		}
		kwargs[kw.Name] = i.eval(kw.Value, env)
	}

	if attr, ok := n.Callee.(*ast.AttrExpr); ok {
		obj := i.eval(attr.Obj, env)
		if inst, ok := obj.(*Instance); ok {
			return i.call(i.getAttr(inst, attr.Name), args, kwargs)
		}
		if cls, ok := obj.(*Class); ok {
			if fn, ok := cls.Methods[attr.Name]; ok {
				return i.call(fn, args, kwargs)
			}
		}
		return i.callMethod(obj, attr.Name, args, kwargs)
	}
	callee := i.eval(n.Callee, env)
	return i.call(callee, args, kwargs)
}

func (i *Interp) call(callee Value, args []Value, kwargs map[string]Value) Value {
	switch fn := callee.(type) {
	case *Native:
		v, err := fn.Fn(i, args, kwargs)
		if err != nil {
			i.throw("%s", err.Error())
		}
		return v
	case *Function:
		return i.callFunction(fn, args, kwargs)
	case *Class:
		return i.instantiate(fn, args, kwargs)
	default:
		i.throw("%s 不能像函数一样调用", TypeName(callee))
	}
	return nil
}

func (i *Interp) instantiate(cls *Class, args []Value, kwargs map[string]Value) Value {
	inst := &Instance{Class: cls, Fields: NewMap()}
	if init, ok := cls.Methods["init"]; ok {
		bound := *init
		bound.Self = inst
		i.callFunction(&bound, args, kwargs)
	} else if len(args) > 0 || len(kwargs) > 0 {
		i.throw("类 %s 没有 init 方法，不能接收构造参数", cls.Name)
	}
	return inst
}

func (i *Interp) getAttr(inst *Instance, name string) Value {
	if v, ok := inst.Fields.Get(name); ok {
		return v
	}
	if fn, ok := inst.Class.Methods[name]; ok {
		bound := *fn
		bound.Self = inst
		return &bound
	}
	i.throw("%s 没有属性或方法 %q", inst.Class.Name, name)
	return nil
}

func (i *Interp) callFunction(fn *Function, args []Value, kwargs map[string]Value) Value {
	params := fn.Params
	if fn.Self != nil {
		if len(params) > 0 && params[0].Name == "self" {
			params = params[1:]
		}
	}
	if len(args) > len(params) {
		i.throw("%s() 最多接受 %d 个参数，收到 %d 个", fn.Name, len(params), len(args))
	}
	callEnv := NewEnv(fn.Env)
	if fn.Self != nil {
		callEnv.Define("self", fn.Self)
	}
	for idx, arg := range args {
		callEnv.Define(params[idx].Name, arg)
	}
	for name, val := range kwargs {
		found := -1
		for idx, p := range params {
			if p.Name == name {
				found = idx
				break
			}
		}
		if found < 0 {
			i.throw("%s() 没有名为 %q 的参数", fn.Name, name)
		}
		if found < len(args) {
			i.throw("参数 %q 同时用位置和关键字传入", name)
		}
		if _, exists := callEnv.Local(name); exists {
			i.throw("参数 %q 重复传入", name)
		}
		callEnv.Define(name, val)
	}
	for _, p := range params {
		if _, ok := callEnv.Local(p.Name); ok {
			continue
		}
		if p.Default == nil {
			i.throw("%s() 缺少参数 %q", fn.Name, p.Name)
		}
		callEnv.Define(p.Name, i.eval(p.Default, callEnv))
	}

	if i.callDepth >= i.maxDepth {
		i.throw("函数调用层数太深（超过 %d 层），可能是无限递归", i.maxDepth)
	}
	callLine := i.line
	i.callDepth++
	i.stack = append(i.stack, fmt.Sprintf("%s() 第 %d 行", fn.Name, callLine))
	defer func() {
		i.callDepth--
		i.stack = i.stack[:len(i.stack)-1]
	}()

	var result Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				if rs, ok := r.(returnSignal); ok {
					result = rs.value
					return
				}
				panic(r)
			}
		}()
		i.execBlock(fn.Body, callEnv)
	}()
	return result
}

// ---- operators ----

func (i *Interp) binary(op token.Type, l, r Value) Value {
	switch op {
	case token.PLUS:
		if lf, ok := asNumber(l); ok {
			if rf, ok := asNumber(r); ok {
				return lf + rf
			}
		}
		if ls, ok := l.(string); ok {
			return ls + ToString(r)
		}
		if _, ok := r.(string); ok {
			return ToString(l) + ToString(r)
		}
		if ll, ok := l.(*List); ok {
			if rl, ok := r.(*List); ok {
				items := make([]Value, 0, len(ll.Items)+len(rl.Items))
				items = append(items, ll.Items...)
				items = append(items, rl.Items...)
				return &List{Items: items}
			}
		}
		if lm, ok := l.(*Map); ok {
			if rm, ok := r.(*Map); ok {
				out := NewMap()
				for _, k := range lm.Keys {
					out.Set(k, lm.M[k])
				}
				for _, k := range rm.Keys {
					out.Set(k, rm.M[k])
				}
				return out
			}
		}
		i.throw("不能把 %s 和 %s 相加", TypeName(l), TypeName(r))

	case token.MINUS, token.STAR, token.SLASH, token.PERCENT:
		if op == token.STAR {
			if s, ok := l.(string); ok {
				if n, ok := asInt(r); ok {
					if n < 0 {
						return ""
					}
					if n > 1_000_000 {
						i.throw("字符串重复次数太大")
					}
					return strings.Repeat(s, n)
				}
			}
			if list, ok := l.(*List); ok {
				if n, ok := asInt(r); ok {
					if n < 0 {
						return &List{}
					}
					if n*len(list.Items) > 10_000_000 {
						i.throw("列表重复次数太大")
					}
					items := make([]Value, 0, n*len(list.Items))
					for k := 0; k < n; k++ {
						items = append(items, list.Items...)
					}
					return &List{Items: items}
				}
			}
		}
		lf, lok := asNumber(l)
		rf, rok := asNumber(r)
		if !lok || !rok {
			i.throw("运算 %s 需要两个数字，收到 %s 和 %s", op, TypeName(l), TypeName(r))
		}
		switch op {
		case token.MINUS:
			return lf - rf
		case token.STAR:
			return lf * rf
		case token.SLASH:
			if rf == 0 {
				i.throw("不能除以零")
			}
			return lf / rf
		case token.PERCENT:
			if rf == 0 {
				i.throw("不能对零取余")
			}
			return math.Mod(lf, rf)
		}

	case token.EQ:
		return Equal(l, r)
	case token.NE:
		return !Equal(l, r)

	case token.LT, token.LE, token.GT, token.GE:
		if lf, ok := asNumber(l); ok {
			rf, ok := asNumber(r)
			if !ok {
				i.throw("不能把数字和 %s 比较", TypeName(r))
			}
			switch op {
			case token.LT:
				return lf < rf
			case token.LE:
				return lf <= rf
			case token.GT:
				return lf > rf
			case token.GE:
				return lf >= rf
			}
		}
		if ls, ok := l.(string); ok {
			rs, ok := asString(r)
			if !ok {
				i.throw("不能把字符串和 %s 比较", TypeName(r))
			}
			switch op {
			case token.LT:
				return ls < rs
			case token.LE:
				return ls <= rs
			case token.GT:
				return ls > rs
			case token.GE:
				return ls >= rs
			}
		}
		i.throw("类型 %s 不支持大小比较", TypeName(l))

	case token.IN:
		return i.contains(l, r)
	}
	i.throw("未知运算符 %s", op)
	return nil
}

func (i *Interp) contains(item, container Value) Value {
	switch c := container.(type) {
	case *List:
		for _, v := range c.Items {
			if Equal(v, item) {
				return true
			}
		}
		return false
	case *Map:
		k, ok := keyString(item)
		if !ok {
			return false
		}
		_, ok = c.Get(k)
		return ok
	case string:
		s, ok := asString(item)
		if !ok {
			i.throw("字符串只能判断子串，不能判断 %s", TypeName(item))
		}
		return strings.Contains(c, s)
	case nil:
		return false
	}
	i.throw("不能对 %s 使用 in", TypeName(container))
	return false
}

// ---- indexing ----

func (i *Interp) slice(obj, lo, hi, step Value, hasLo, hasHi, hasStep bool) Value {
	stepN := 1
	if hasStep {
		var ok bool
		stepN, ok = asInt(step)
		if !ok {
			i.throw("切片步长必须是整数")
		}
		if stepN == 0 {
			i.throw("切片步长不能为 0")
		}
	}
	pick := func(o *List, runes []rune, n int) Value {
		start, end := 0, n
		if stepN < 0 {
			start, end = n-1, -1
		}
		index := func(v Value) int {
			idx, ok := asInt(v)
			if !ok {
				i.throw("切片下标必须是整数")
			}
			if idx < 0 {
				idx += n
			}
			return idx
		}
		if hasLo {
			start = index(lo)
		}
		if hasHi {
			end = index(hi)
		}
		if stepN > 0 {
			if start < 0 {
				start = 0
			}
			if start > n {
				start = n
			}
			if end < 0 {
				end = 0
			}
			if end > n {
				end = n
			}
		} else {
			if start >= n {
				start = n - 1
			}
			if start < -1 {
				start = -1
			}
			if end >= n {
				end = n - 1
			}
			if end < -1 {
				end = -1
			}
		}
		if stepN == 1 {
			if runes != nil {
				return string(runes[start:end])
			}
			items := make([]Value, end-start)
			copy(items, o.Items[start:end])
			return &List{Items: items}
		}
		if runes != nil {
			var b strings.Builder
			for idx := start; (stepN > 0 && idx < end) || (stepN < 0 && idx > end); idx += stepN {
				b.WriteRune(runes[idx])
			}
			return b.String()
		}
		items := make([]Value, 0)
		for idx := start; (stepN > 0 && idx < end) || (stepN < 0 && idx > end); idx += stepN {
			items = append(items, o.Items[idx])
		}
		return &List{Items: items}
	}
	switch o := obj.(type) {
	case *List:
		return pick(o, nil, len(o.Items))
	case string:
		return pick(nil, []rune(o), len([]rune(o)))
	default:
		i.throw("%s 不支持切片", TypeName(obj))
	}
	return nil
}

func (i *Interp) index(obj, key Value) Value {
	switch o := obj.(type) {
	case *List:
		idx, ok := asInt(key)
		if !ok {
			i.throw("列表下标必须是整数")
		}
		if idx < 0 {
			idx += len(o.Items)
		}
		if idx < 0 || idx >= len(o.Items) {
			i.throw("列表下标 %s 越界（长度 %d）", ToString(key), len(o.Items))
		}
		return o.Items[idx]
	case string:
		runes := []rune(o)
		idx, ok := asInt(key)
		if !ok {
			i.throw("字符串下标必须是整数")
		}
		if idx < 0 {
			idx += len(runes)
		}
		if idx < 0 || idx >= len(runes) {
			i.throw("字符串下标 %s 越界（长度 %d）", ToString(key), len(runes))
		}
		return string(runes[idx])
	case *Map:
		k, ok := keyString(key)
		if !ok {
			i.throw("字典的键必须是字符串或数字")
		}
		v, ok := o.Get(k)
		if !ok {
			i.throw("字典中不存在键 %q", k)
		}
		return v
	case nil:
		i.throw("null 不能使用下标")
	default:
		i.throw("%s 不能使用下标", TypeName(obj))
	}
	return nil
}

func (i *Interp) setIndex(obj, key, val Value) {
	switch o := obj.(type) {
	case *List:
		idx, ok := asInt(key)
		if !ok {
			i.throw("列表下标必须是整数")
		}
		if idx < 0 {
			idx += len(o.Items)
		}
		if idx < 0 || idx >= len(o.Items) {
			i.throw("列表下标 %s 越界（长度 %d）", ToString(key), len(o.Items))
		}
		o.Items[idx] = val
	case *Map:
		k, ok := keyString(key)
		if !ok {
			i.throw("字典的键必须是字符串或数字")
		}
		o.Set(k, val)
	default:
		i.throw("%s 不能用下标赋值", TypeName(obj))
	}
}

// ---- iteration ----

func (i *Interp) iterate(v Value) []Value {
	switch x := v.(type) {
	case *List:
		out := make([]Value, len(x.Items))
		copy(out, x.Items)
		return out
	case string:
		runes := []rune(x)
		out := make([]Value, len(runes))
		for idx, r := range runes {
			out[idx] = string(r)
		}
		return out
	case *Map:
		out := make([]Value, len(x.Keys))
		for idx, k := range x.Keys {
			out[idx] = k
		}
		return out
	case float64:
		n, ok := asInt(x)
		if !ok || n < 0 {
			i.throw("for 的数字范围必须是正整数")
		}
		if n > 10_000_000 {
			i.throw("for 的数字范围太大")
		}
		out := make([]Value, n)
		for k := 0; k < n; k++ {
			out[k] = float64(k)
		}
		return out
	case nil:
		i.throw("不能遍历 null")
	}
	i.throw("%s 不能遍历", TypeName(v))
	return nil
}
