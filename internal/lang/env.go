package lang

// Env is a lexical scope.
type Env struct {
	vars   map[string]Value
	parent *Env
}

// NewEnv creates a scope nested in parent (which may be nil).
func NewEnv(parent *Env) *Env {
	return &Env{vars: map[string]Value{}, parent: parent}
}

// Get looks a name up through the scope chain.
func (e *Env) Get(name string) (Value, bool) {
	for s := e; s != nil; s = s.parent {
		if v, ok := s.vars[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// Local reports whether name exists in this exact scope.
func (e *Env) Local(name string) (Value, bool) {
	v, ok := e.vars[name]
	return v, ok
}

// Define binds a name in this scope.
func (e *Env) Define(name string, v Value) {
	e.vars[name] = v
}

// Assign updates an existing binding in any enclosing scope, otherwise it
// defines the name locally. This keeps small scripts friendly: event handlers
// and functions can update top-level counters without a global keyword.
func (e *Env) Assign(name string, v Value) {
	for s := e; s != nil; s = s.parent {
		if _, ok := s.vars[name]; ok {
			s.vars[name] = v
			return
		}
	}
	e.vars[name] = v
}

// Snapshot returns a copy of the variables in this exact scope.
func (e *Env) Snapshot() map[string]Value {
	out := make(map[string]Value, len(e.vars))
	for k, v := range e.vars {
		out[k] = v
	}
	return out
}
