//go:build windows

package lang

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

// SQLite is a thin wrapper around the winsqlite3.dll shipped with Windows.
// It intentionally has no third-party dependency.
type SQLite struct {
	mu      sync.Mutex
	db      uintptr
	path    string
	closed  bool
	lastErr string
}

var (
	sqliteDLL = syscall.NewLazyDLL("winsqlite3.dll")

	sqliteOpenV2        = sqliteDLL.NewProc("sqlite3_open_v2")
	sqliteClose         = sqliteDLL.NewProc("sqlite3_close")
	sqliteErrmsg        = sqliteDLL.NewProc("sqlite3_errmsg")
	sqlitePrepareV2     = sqliteDLL.NewProc("sqlite3_prepare_v2")
	sqliteStep          = sqliteDLL.NewProc("sqlite3_step")
	sqliteFinalize      = sqliteDLL.NewProc("sqlite3_finalize")
	sqliteColumnCount   = sqliteDLL.NewProc("sqlite3_column_count")
	sqliteColumnName    = sqliteDLL.NewProc("sqlite3_column_name")
	sqliteColumnType    = sqliteDLL.NewProc("sqlite3_column_type")
	sqliteColumnInt64   = sqliteDLL.NewProc("sqlite3_column_int64")
	sqliteColumnText    = sqliteDLL.NewProc("sqlite3_column_text")
	sqliteColumnBlob    = sqliteDLL.NewProc("sqlite3_column_blob")
	sqliteColumnBytes   = sqliteDLL.NewProc("sqlite3_column_bytes")
	sqliteBindText      = sqliteDLL.NewProc("sqlite3_bind_text")
	sqliteBindInt64     = sqliteDLL.NewProc("sqlite3_bind_int64")
	sqliteBindBlob      = sqliteDLL.NewProc("sqlite3_bind_blob")
	sqliteBindNull      = sqliteDLL.NewProc("sqlite3_bind_null")
	sqliteBindCount     = sqliteDLL.NewProc("sqlite3_bind_parameter_count")
	sqliteReset         = sqliteDLL.NewProc("sqlite3_reset")
	sqliteClearBindings = sqliteDLL.NewProc("sqlite3_clear_bindings")
	sqliteChanges       = sqliteDLL.NewProc("sqlite3_changes")
	sqliteLastInsertID  = sqliteDLL.NewProc("sqlite3_last_insert_rowid")
	sqliteExec          = sqliteDLL.NewProc("sqlite3_exec")
	sqliteBusyTimeout   = sqliteDLL.NewProc("sqlite3_busy_timeout")
)

const (
	sqliteOK   = 0
	sqliteRow  = 100
	sqliteDone = 101

	sqliteOpenReadOnly  = 0x00000001
	sqliteOpenReadWrite = 0x00000002
	sqliteOpenCreate    = 0x00000004
	sqliteOpenMemory    = 0x00000080
	sqliteTransient     = ^uintptr(0)
)

func sqliteCString(s string) *byte {
	p, err := syscall.BytePtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

func sqliteGoCString(p *byte) string {
	if p == nil {
		return ""
	}
	var b []byte
	for i := 0; i < 65536; i++ {
		c := *(*byte)(unsafe.Add(unsafe.Pointer(p), i))
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b)
}

func sqliteGoString(p *byte, n int32) string {
	if p == nil || n <= 0 {
		return ""
	}
	b := unsafe.Slice((*byte)(unsafe.Pointer(p)), int(n))
	return string(b)
}

func (d *SQLite) err() error {
	if d.db == 0 {
		return fmt.Errorf("数据库已关闭")
	}
	p, _, _ := sqliteErrmsg.Call(d.db)
	msg := sqliteGoCString((*byte)(unsafe.Pointer(p)))
	if msg == "" {
		msg = "SQLite 错误"
	}
	d.lastErr = msg
	return fmt.Errorf("%s", msg)
}

func openSQLite(path string, readonly, create bool) (*SQLite, error) {
	flags := uintptr(sqliteOpenReadWrite)
	if readonly {
		flags = sqliteOpenReadOnly
	}
	if create && !readonly {
		flags |= sqliteOpenCreate
	}
	if path == ":memory:" {
		flags = sqliteOpenReadWrite | sqliteOpenCreate | sqliteOpenMemory
	}
	var db uintptr
	p := sqliteCString(path)
	rc, _, _ := sqliteOpenV2.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&db)), flags, 0)
	runtime.KeepAlive(p)
	if rc != sqliteOK {
		msg := "无法打开数据库"
		if db != 0 {
			ep, _, _ := sqliteErrmsg.Call(db)
			msg = sqliteGoCString((*byte)(unsafe.Pointer(ep)))
			sqliteClose.Call(db)
		}
		return nil, fmt.Errorf("%s", msg)
	}
	d := &SQLite{db: db, path: path}
	sqliteBusyTimeout.Call(db, 3000)
	return d, nil
}

func (d *SQLite) close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed || d.db == 0 {
		return nil
	}
	sqliteClose.Call(d.db)
	d.db = 0
	d.closed = true
	return nil
}

func (d *SQLite) exec(sql string) (int64, error) {
	return d.run(sql, nil)
}

func sqliteBind(stmt uintptr, args []Value) error {
	n, _, _ := sqliteBindCount.Call(stmt)
	if int(n) != len(args) {
		return fmt.Errorf("SQL 需要 %d 个参数，实际 %d 个", n, len(args))
	}
	for idx, v := range args {
		i := uintptr(idx + 1)
		var rc uintptr
		switch t := v.(type) {
		case nil:
			rc, _, _ = sqliteBindNull.Call(stmt, i)
		case float64:
			// syscall.SyscallN cannot pass a double in XMM0; bind as text and let
			// SQLite's column affinity convert it to a number when needed.
			s := strconv.FormatFloat(t, 'g', -1, 64)
			p := sqliteCString(s)
			rc, _, _ = sqliteBindText.Call(stmt, i, uintptr(unsafe.Pointer(p)), uintptr(len(s)), sqliteTransient)
			runtime.KeepAlive(p)
		case bool:
			if t {
				rc, _, _ = sqliteBindInt64.Call(stmt, i, 1)
			} else {
				rc, _, _ = sqliteBindInt64.Call(stmt, i, 0)
			}
		case string:
			p := sqliteCString(t)
			rc, _, _ = sqliteBindText.Call(stmt, i, uintptr(unsafe.Pointer(p)), uintptr(len(t)), sqliteTransient)
			runtime.KeepAlive(p)
		case *Bytes:
			if len(t.Data) == 0 {
				rc, _, _ = sqliteBindBlob.Call(stmt, i, 0, 0, sqliteTransient)
			} else {
				rc, _, _ = sqliteBindBlob.Call(stmt, i, uintptr(unsafe.Pointer(&t.Data[0])), uintptr(len(t.Data)), sqliteTransient)
			}
		default:
			s := ToString(v)
			p := sqliteCString(s)
			rc, _, _ = sqliteBindText.Call(stmt, i, uintptr(unsafe.Pointer(p)), uintptr(len(s)), sqliteTransient)
			runtime.KeepAlive(p)
		}
		if rc != sqliteOK {
			return fmt.Errorf("绑定第 %d 个参数失败", idx+1)
		}
	}
	return nil
}

func (d *SQLite) run(sql string, args []Value) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.db == 0 {
		return 0, fmt.Errorf("数据库已关闭")
	}
	p := sqliteCString(sql)
	var stmt uintptr
	rc, _, _ := sqlitePrepareV2.Call(d.db, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&stmt)), 0)
	runtime.KeepAlive(p)
	if rc != sqliteOK {
		return 0, d.err()
	}
	defer sqliteFinalize.Call(stmt)
	if err := sqliteBind(stmt, args); err != nil {
		return 0, err
	}
	for {
		rc, _, _ = sqliteStep.Call(stmt)
		if rc == sqliteDone {
			break
		}
		if rc != sqliteRow {
			return 0, d.err()
		}
	}
	ch, _, _ := sqliteChanges.Call(d.db)
	return int64(ch), nil
}

func (d *SQLite) query(sql string, args []Value) (*List, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.queryLocked(sql, args)
}

func (d *SQLite) queryLocked(sql string, args []Value) (*List, error) {
	if d.db == 0 {
		return nil, fmt.Errorf("数据库已关闭")
	}
	p := sqliteCString(sql)
	var stmt uintptr
	rc, _, _ := sqlitePrepareV2.Call(d.db, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&stmt)), 0)
	runtime.KeepAlive(p)
	if rc != sqliteOK {
		return nil, d.err()
	}
	defer sqliteFinalize.Call(stmt)
	if err := sqliteBind(stmt, args); err != nil {
		return nil, err
	}
	cols, _, _ := sqliteColumnCount.Call(stmt)
	names := make([]string, int(cols))
	for i := 0; i < int(cols); i++ {
		np, _, _ := sqliteColumnName.Call(stmt, uintptr(i))
		names[i] = sqliteGoCString((*byte)(unsafe.Pointer(np)))
	}
	out := &List{}
	for {
		rc, _, _ = sqliteStep.Call(stmt)
		if rc == sqliteDone {
			break
		}
		if rc != sqliteRow {
			return nil, d.err()
		}
		row := NewMap()
		for i := 0; i < int(cols); i++ {
			typ, _, _ := sqliteColumnType.Call(stmt, uintptr(i))
			switch int(typ) {
			case 1:
				v, _, _ := sqliteColumnInt64.Call(stmt, uintptr(i))
				row.Set(names[i], float64(int64(v)))
			case 2:
				// sqlite3_column_double returns a double in XMM0, which SyscallN
				// cannot read; use SQLite's text conversion instead.
				tp, _, _ := sqliteColumnText.Call(stmt, uintptr(i))
				n, _, _ := sqliteColumnBytes.Call(stmt, uintptr(i))
				f, _ := strconv.ParseFloat(sqliteGoString((*byte)(unsafe.Pointer(tp)), int32(n)), 64)
				row.Set(names[i], f)
			case 3:
				tp, _, _ := sqliteColumnText.Call(stmt, uintptr(i))
				n, _, _ := sqliteColumnBytes.Call(stmt, uintptr(i))
				row.Set(names[i], sqliteGoString((*byte)(unsafe.Pointer(tp)), int32(n)))
			case 4:
				bp, _, _ := sqliteColumnBlob.Call(stmt, uintptr(i))
				n, _, _ := sqliteColumnBytes.Call(stmt, uintptr(i))
				data := unsafe.Slice((*byte)(unsafe.Pointer(bp)), int(n))
				row.Set(names[i], NewBytes(append([]byte(nil), data...)))
			default:
				row.Set(names[i], nil)
			}
		}
		out.Items = append(out.Items, row)
	}
	return out, nil
}

func (d *SQLite) Kind() string { return "sqlite" }

func (d *SQLite) Describe() string { return "SQLite 数据库" }

func (d *SQLite) GetProp(name string) (Value, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch name {
	case "path":
		return d.path, true
	case "closed":
		return d.closed, true
	case "last_error":
		return d.lastErr, true
	case "tables":
		list, err := d.queryLocked("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name", nil)
		if err != nil {
			return nil, false
		}
		out := &List{}
		for _, row := range list.Items {
			if m, ok := row.(*Map); ok {
				out.Items = append(out.Items, m.M["name"])
			}
		}
		return out, true
	}
	return nil, false
}

func (d *SQLite) SetProp(name string, v Value) error {
	return fmt.Errorf("数据库没有可写属性 %q", name)
}

func (d *SQLite) Listen(event string, fn func() error) error {
	return fmt.Errorf("数据库不支持事件")
}

func (d *SQLite) CallMethod(name string, args []Value, kwargs map[string]Value) (Value, bool, error) {
	switch name {
	case "exec":
		if len(args) != 1 {
			return nil, true, fmt.Errorf("exec(sql) 需要 1 个参数")
		}
		n, err := d.exec(ToString(args[0]))
		return float64(n), true, err
	case "execute":
		if len(args) < 1 {
			return nil, true, fmt.Errorf("execute(sql, ...) 需要 SQL")
		}
		n, err := d.run(ToString(args[0]), args[1:])
		if err != nil {
			return nil, true, err
		}
		id, _, _ := sqliteLastInsertID.Call(d.db)
		m := NewMap()
		m.Set("changes", float64(n))
		m.Set("last_id", float64(int64(id)))
		return m, true, nil
	case "query":
		if len(args) < 1 {
			return nil, true, fmt.Errorf("query(sql, ...) 需要 SQL")
		}
		list, err := d.query(ToString(args[0]), args[1:])
		return list, true, err
	case "close":
		return nil, true, d.close()
	case "tables":
		list, err := d.query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name", nil)
		if err != nil {
			return nil, true, err
		}
		out := &List{}
		for _, row := range list.Items {
			if m, ok := row.(*Map); ok {
				out.Items = append(out.Items, m.M["name"])
			}
		}
		return out, true, nil
	}
	return nil, false, nil
}

func registerSQLite(i *Interp) {
	i.RegisterNative("sqlite_open", func(_ *Interp, args []Value, kwargs map[string]Value) (Value, error) {
		path := ToString(Pos(args, 0, ":memory:"))
		readonly := false
		create := true
		if v, ok := kwargs["readonly"]; ok {
			readonly = Truthy(v)
		}
		if v, ok := kwargs["create"]; ok {
			create = Truthy(v)
		}
		return openSQLite(path, readonly, create)
	})
}
