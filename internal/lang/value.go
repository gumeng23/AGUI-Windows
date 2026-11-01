// Package lang implements the Agui runtime: values, environments, the tree
// walking interpreter and the core built-in functions.
package lang

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"agui/internal/ast"
)

// Value is any Agui runtime value: null, bool, float64, string, *List, *Map,
// *Function, *Native, a GUI widget or a GUI font.
type Value any

// List is a mutable Agui list.
type List struct {
	Items []Value
}

// NewList builds a list from values.
func NewList(items ...Value) *List { return &List{Items: items} }

// Len returns the number of items.
func (l *List) Len() int { return len(l.Items) }

// Class is a lightweight Agui class definition created by `class Name:`.
type Class struct {
	Name    string
	Methods map[string]*Function
}

// Describe implements Describer.
func (c *Class) Describe() string { return "<类 " + c.Name + ">" }

// Instance is one object created from a Class.
type Instance struct {
	Class  *Class
	Fields *Map
}

// Describe implements Describer.
func (inst *Instance) Describe() string {
	if inst == nil || inst.Class == nil {
		return "<对象>"
	}
	return "<" + inst.Class.Name + " 对象>"
}

// Map is an insertion-ordered Agui dictionary with string keys.
type Map struct {
	Keys []string
	M    map[string]Value
}

// NewMap builds an empty map.
func NewMap() *Map { return &Map{M: map[string]Value{}} }

// Get returns the value for key.
func (m *Map) Get(key string) (Value, bool) {
	v, ok := m.M[key]
	return v, ok
}

// Set inserts or updates a key.
func (m *Map) Set(key string, v Value) {
	if _, ok := m.M[key]; !ok {
		m.Keys = append(m.Keys, key)
	}
	m.M[key] = v
}

// Delete removes a key.
func (m *Map) Delete(key string) bool {
	if _, ok := m.M[key]; !ok {
		return false
	}
	delete(m.M, key)
	for idx, k := range m.Keys {
		if k == key {
			m.Keys = append(m.Keys[:idx], m.Keys[idx+1:]...)
			break
		}
	}
	return true
}

// Function is a user defined Agui function.
type Function struct {
	Name   string
	Params []ast.Param
	Body   []ast.Stmt
	Env    *Env
	Line   int
	// Self is set on bound methods; callFunction binds it as `self`.
	Self Value
}

// Native is a built-in function implemented in Go.
type Native struct {
	Name string
	Fn   func(i *Interp, args []Value, kwargs map[string]Value) (Value, error)
}

// Widget is implemented by every GUI control. The language core stays free of
// Win32 details through this interface.
type Widget interface {
	// Kind returns the widget kind, e.g. "button" or "window".
	Kind() string
	// GetProp returns a readable property.
	GetProp(name string) (Value, bool)
	// SetProp writes a property.
	SetProp(name string, v Value) error
	// CallMethod handles widget methods such as win.close().
	CallMethod(name string, args []Value, kwargs map[string]Value) (Value, bool, error)
	// Listen registers an event handler. The callback returns an error which the
	// GUI reports to the user.
	Listen(event string, fn func() error) error
}

// Describer lets a host value add a short human readable description.
type Describer interface {
	Describe() string
}

// Truthy implements Agui truthiness.
func Truthy(v Value) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case *List:
		return len(x.Items) > 0
	case *Map:
		return len(x.M) > 0
	default:
		return true
	}
}

// Equal compares two values deeply.
func Equal(a, b Value) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case float64:
		y, ok := b.(float64)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case *List:
		y, ok := b.(*List)
		if !ok || len(x.Items) != len(y.Items) {
			return false
		}
		for idx := range x.Items {
			if !Equal(x.Items[idx], y.Items[idx]) {
				return false
			}
		}
		return true
	case *Map:
		y, ok := b.(*Map)
		if !ok || len(x.M) != len(y.M) {
			return false
		}
		for _, k := range x.Keys {
			yv, ok := y.M[k]
			if !ok || !Equal(x.M[k], yv) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

// TypeName returns the Agui type name used by type().
func TypeName(v Value) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case *List:
		return "list"
	case *Map:
		return "map"
	case *Class:
		return "class"
	case *Instance:
		return "object"
	case *Function, *Native:
		return "function"
	default:
		if _, ok := v.(Widget); ok {
			return "widget"
		}
		if _, ok := v.(*Font); ok {
			return "font"
		}
		return "value"
	}
}

// Font is a UI font handle chosen with agui.font(...). The actual platform
// handle lives in the GUI package; the runtime only carries it as a value.
type Font struct {
	Family string
	Size   float64
	Bold   bool
	Italic bool
	// Handle is filled in by the GUI host.
	Handle any
}

func (f *Font) Describe() string {
	return fmt.Sprintf("<font %s %g>", f.Family, f.Size)
}

// ToString converts a value for display and for string concatenation.
func ToString(v Value) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return formatNumber(x)
	case string:
		return x
	case *List:
		parts := make([]string, len(x.Items))
		for idx, item := range x.Items {
			parts[idx] = repr(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Map:
		parts := make([]string, 0, len(x.Keys))
		for _, k := range x.Keys {
			parts = append(parts, strconv.Quote(k)+": "+repr(x.M[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *Class:
		return "<类 " + x.Name + ">"
	case *Instance:
		return x.Describe()
	case *Function:
		if x.Name == "" {
			return "<函数>"
		}
		return "<函数 " + x.Name + ">"
	case *Native:
		return "<内置函数 " + x.Name + ">"
	default:
		if d, ok := v.(Describer); ok {
			return d.Describe()
		}
		if w, ok := v.(Widget); ok {
			return "<控件 " + w.Kind() + ">"
		}
		return fmt.Sprintf("%v", v)
	}
}

func repr(v Value) string {
	if s, ok := v.(string); ok {
		return strconv.QuoteToASCII(s)
	}
	return ToString(v)
}

func formatNumber(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func asNumber(v Value) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

func asInt(v Value) (int, bool) {
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) || math.Abs(f) > 1e15 {
		return 0, false
	}
	return int(f), true
}

func asString(v Value) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func keyString(v Value) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case float64:
		return formatNumber(x), true
	case bool:
		return ToString(x), true
	}
	return "", false
}

// Pos returns a positional argument or a default.
func Pos(args []Value, idx int, def Value) Value {
	if idx < len(args) {
		return args[idx]
	}
	return def
}

// Named reads a keyword argument, falling back to the positional slot, then to
// the default. It lets built-ins accept both `edit("x", w=200)` styles.
func Named(args []Value, kwargs map[string]Value, pos int, name string, def Value) Value {
	if v, ok := kwargs[name]; ok {
		return v
	}
	return Pos(args, pos, def)
}

// CheckArity rejects extraneous positional arguments.
func CheckArity(name string, args []Value, max int) error {
	if len(args) > max {
		return fmt.Errorf("%s(): 最多接受 %d 个位置参数，收到 %d 个", name, max, len(args))
	}
	return nil
}
