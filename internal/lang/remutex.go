package lang

import (
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// reentrantMutex is a mutex that can be locked repeatedly by the same
// goroutine. GUI event callbacks run inside the message loop that is itself
// called from run(), so nested Protect calls on the UI thread must not
// deadlock. Other goroutines (background tasks) still wait.
type reentrantMutex struct {
	mu    sync.Mutex
	owner uint64
	depth int
}

func currentGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// The first line is "goroutine 123 [running]:".
	fields := strings.Fields(string(buf[:n]))
	if len(fields) >= 2 {
		if id, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			return id
		}
	}
	return 0
}

func (m *reentrantMutex) Lock() {
	gid := currentGoroutineID()
	if m.owner == gid {
		m.depth++
		return
	}
	m.mu.Lock()
	m.owner = gid
	m.depth = 1
}

func (m *reentrantMutex) Unlock() {
	gid := currentGoroutineID()
	if m.owner != gid {
		panic("agui: 解锁了不属于当前协程的锁")
	}
	m.depth--
	if m.depth <= 0 {
		m.owner = 0
		m.depth = 0
		m.mu.Unlock()
	}
}

// Suspend releases the lock before entering a blocking message loop. It
// returns the nesting depth, which must be passed to Resume.
func (m *reentrantMutex) Suspend() int {
	gid := currentGoroutineID()
	if m.owner != gid {
		return 0
	}
	depth := m.depth
	m.depth = 0
	m.owner = 0
	m.mu.Unlock()
	return depth
}

// Resume reacquires a lock released by Suspend, waiting for any UI callback or
// background task that ran while the loop was active.
func (m *reentrantMutex) Resume(depth int) {
	if depth <= 0 {
		return
	}
	m.mu.Lock()
	m.owner = currentGoroutineID()
	m.depth = depth
}
