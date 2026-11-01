package lang

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task is a cancellable background job created by async().
//
// The interpreter is cooperative: only one goroutine executes script code at a
// time, but a task releases the interpreter lock at statement boundaries and
// around blocking calls. This keeps the UI responsive without allowing two
// goroutines to mutate the same environment concurrently.
type Task struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once

	mu        sync.Mutex
	result    Value
	err       error
	cancelled bool
	started   time.Time
	finished  time.Time
}

// NewTask creates a pending task.
func NewTask() *Task {
	ctx, cancel := context.WithCancel(context.Background())
	return &Task{
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
		started: time.Now(),
	}
}

// Context is used by blocking built-ins so cancel() can interrupt them.
func (t *Task) Context() context.Context { return t.ctx }

// Cancel asks the task to stop at the next statement boundary or blocking call.
func (t *Task) Cancel() {
	t.mu.Lock()
	if !t.cancelled {
		t.cancelled = true
	}
	t.mu.Unlock()
	t.cancel()
}

// IsCancelled reports whether Cancel was called.
func (t *Task) IsCancelled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cancelled
}

// Finish records the result and closes done. It is safe to call once.
func (t *Task) Finish(v Value, err error) {
	t.once.Do(func() {
		t.mu.Lock()
		t.result = v
		t.err = err
		if t.ctx.Err() != nil {
			t.cancelled = true
		}
		t.finished = time.Now()
		t.mu.Unlock()
		close(t.done)
	})
}

// Done returns a channel closed when the task finishes.
func (t *Task) Done() <-chan struct{} { return t.done }

// Wait blocks until the task finishes or the timeout expires. A non-positive
// timeout waits indefinitely. It returns true when the task has finished.
func (t *Task) Wait(timeout time.Duration) bool {
	if timeout <= 0 {
		<-t.done
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-t.done:
		return true
	case <-timer.C:
		return false
	}
}

// Result returns the task result (nil until finished).
func (t *Task) Result() Value {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.result
}

// Err returns the task error, if any.
func (t *Task) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// Status returns running, done, failed or cancelled.
func (t *Task) Status() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	select {
	case <-t.done:
		if t.cancelled {
			return "cancelled"
		}
		if t.err != nil {
			return "failed"
		}
		return "done"
	default:
		return "running"
	}
}

// GetProp implements property access for task values.
func (t *Task) GetProp(name string) (Value, bool) {
	switch name {
	case "status":
		return t.Status(), true
	case "done", "finished":
		select {
		case <-t.done:
			return true, true
		default:
			return false, true
		}
	case "cancelled":
		return t.IsCancelled(), true
	case "result":
		return t.Result(), true
	case "error":
		if err := t.Err(); err != nil {
			return err.Error(), true
		}
		return "", true
	default:
		return nil, false
	}
}

// CallMethod implements task methods.
func (t *Task) CallMethod(i *Interp, name string, args []Value, kwargs map[string]Value) (Value, bool, error) {
	if len(kwargs) > 0 {
		return nil, true, fmt.Errorf("任务方法 %s 不支持关键字参数", name)
	}
	switch name {
	case "cancel":
		if len(args) > 0 {
			return nil, true, fmt.Errorf("task.cancel() 不接受参数")
		}
		t.Cancel()
		return nil, true, nil
	case "wait":
		timeout := 5 * time.Second
		if len(args) > 1 {
			return nil, true, fmt.Errorf("task.wait(超时秒数?) 最多接受 1 个参数")
		}
		if len(args) == 1 {
			f, ok := asNumber(args[0])
			if !ok || f < 0 {
				return nil, true, fmt.Errorf("task.wait(): 超时必须是秒数")
			}
			if f == 0 {
				timeout = 0
			} else {
				timeout = time.Duration(f * float64(time.Second))
			}
		}
		depth := i.Suspend()
		ok := t.Wait(timeout)
		i.Resume(depth)
		return ok, true, nil
	case "result":
		if len(args) > 0 {
			return nil, true, fmt.Errorf("task.result() 不接受参数")
		}
		return t.Result(), true, nil
	}
	return nil, false, nil
}

// Describe implements Describer.
func (t *Task) Describe() string {
	return "<任务 " + t.Status() + ">"
}
