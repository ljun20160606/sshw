package sshwctl

import (
	"errors"
	"io"
	"regexp"
	"sync"
	"time"
)

const (
	KeyCallback = "callback"
)

func init() {
	_ = bus.Subscribe(OnStdout, CallbackOnStdout)
	_ = bus.Subscribe(PostShell, CallbackPostShell)
}

type CallbackInfo struct {
	IsError bool
	Index   int
	Mutex   *sync.Mutex
}

func NewCallbackInfo() *CallbackInfo {
	mutex := new(sync.Mutex)
	lifecycleCallback := &CallbackInfo{
		Mutex: mutex,
	}
	return lifecycleCallback
}

func CallbackOnStdout(ctx *EventContext, line []byte) error {
	node := ctx.Node
	callback, _ := ctx.Get(KeyCallback)
	l := callback.(*CallbackInfo)

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

func CallbackPostShell(ctx *EventContext, stdin io.WriteCloser) error {
	node := ctx.Node
	callback, _ := ctx.Get(KeyCallback)
	l := callback.(*CallbackInfo)

	for i := range node.CallbackShells {
		l.Mutex.Lock()
		if l.IsError {
			l.Mutex.Unlock()
			return errors.New("interrupt")
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
