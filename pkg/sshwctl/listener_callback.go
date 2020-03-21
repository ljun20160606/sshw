package sshwctl

import (
	"io"
	"regexp"
	"sync"
	"time"
)

func init() {
	mutex := new(sync.Mutex)
	lifecycleCallback := &LifecycleCallback{
		Mutex: mutex,
	}
	RegisterLifecycle(&CommonLifecycle{
		Name:          "callback",
		PostShellFunc: lifecycleCallback.PostShell,
		OnStdoutFunc:  lifecycleCallback.OnStdout,
	})
}

type LifecycleCallback struct {
	IsError bool
	Index   int
	Mutex   *sync.Mutex
	Cond    *sync.Cond
}

func (l *LifecycleCallback) OnStdout(node *Node, line []byte) error {
	l.Mutex.Lock()
	defer l.Mutex.Unlock()
	if len(node.CallbackShells) == 0 || l.Index == len(node.CallbackShells)-1 {
		return nil
	}
	shell := node.CallbackShells[l.Index]
	pattern := shell.ErrorPattern
	if pattern != "" {
		s := string(line)
		matched, err := regexp.MatchString(pattern, s)
		if err != nil {
			return err
		}
		if matched {
			l.IsError = true
		}
	}
	return nil
}

func (l *LifecycleCallback) PostShell(node *Node, stdin io.WriteCloser) error {
	for i := range node.CallbackShells {
		l.Mutex.Lock()
		if l.IsError {
			l.Mutex.Unlock()
			return ErrorInterrupt
		}
		l.Mutex.Unlock()
		shell := node.CallbackShells[i]
		// delay
		time.Sleep(shell.Delay * time.Millisecond)
		// Cmd Shell
		_, _ = stdin.Write([]byte(shell.Cmd + "\r"))

		l.Mutex.Lock()
		l.Index = i
		l.Mutex.Unlock()
		// wait
		time.Sleep(shell.Wait * time.Millisecond)
		// wait error
		if shell.ErrorPattern != "" {
			time.Sleep(1000 * time.Millisecond)
		}
	}
	return nil
}
