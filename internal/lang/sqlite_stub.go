//go:build !windows

package lang

// SQLite is unavailable on non-Windows hosts in this build. The WebView
// runtime intentionally keeps the core portable; game saves use the browser
// storage API instead.
type SQLite struct{}

func registerSQLite(i *Interp) {}
