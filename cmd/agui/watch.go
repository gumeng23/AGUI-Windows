package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"
)

// watchFile runs a script and restarts it whenever the file changes.
func watchFile(path string, scriptArgs []string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	var child *exec.Cmd
	start := func() {
		cmd := exec.Command(exe, append([]string{"run", abs}, scriptArgs...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "启动失败："+err.Error())
			return
		}
		child = cmd
		fmt.Printf("[watch] 已启动 %s (pid %d)\n", filepath.Base(abs), cmd.Process.Pid)
	}
	stop := func() {
		if child != nil && child.Process != nil {
			_ = child.Process.Kill()
			_, _ = child.Process.Wait()
			child = nil
		}
	}
	defer stop()
	start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	stat, _ := os.Stat(abs)
	last := stat.ModTime()
	ticker := time.NewTicker(350 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-sig:
			return nil
		case <-ticker.C:
			st, err := os.Stat(abs)
			if err != nil || !st.ModTime().After(last) {
				continue
			}
			last = st.ModTime()
			fmt.Printf("[watch] 检测到修改，重启\n")
			stop()
			time.Sleep(80 * time.Millisecond)
			start()
		}
	}
}
