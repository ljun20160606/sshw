package sshwctl

import (
	"container/heap"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"io"
)

type Lifecycle interface {
	PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error

	PostSSHDial(node *Node, client *ssh.Client) error

	PostNewSession(node *Node, session *ssh.Session) error

	PostShell(node *Node, stdin io.WriteCloser) error

	OnStdout(node *Node, line []byte) error

	OnStderr(node *Node, line []byte) error

	PostSessionWait(node *Node) error

	Priority() int
}

var (
	ErrorInterrupt = errors.New("interrupt")
)

func NewLifeCycleComposite() *LifecycleComposite {
	l := new(LifecycleComposite)
	for i := range lifecycleInitial {
		f := lifecycleInitial[i]
		f(&l.queue)
	}
	return l
}

func RegisterLifecycle(lifecycle Lifecycle) {
	lifecycleInitial = append(lifecycleInitial, func(queue *LifecycleQueue) {
		heap.Push(queue, lifecycle)
	})
}

var _ Lifecycle = new(LifecycleComposite)

var lifecycleInitial []func(queue *LifecycleQueue)

type LifecycleComposite struct {
	queue LifecycleQueue
}

func (l *LifecycleComposite) Priority() int {
	return 0
}

type LifecycleQueue []Lifecycle

func (l *LifecycleQueue) Len() int {
	return len(*l)
}

func (l *LifecycleQueue) Less(i, j int) bool {
	return (*l)[i].Priority() > (*l)[j].Priority()
}

func (l *LifecycleQueue) Swap(i, j int) {
	(*l)[i], (*l)[j] = (*l)[j], (*l)[i]
}

func (l *LifecycleQueue) Push(x interface{}) {
	*l = append(*l, x.(Lifecycle))
}

func (l *LifecycleQueue) Pop() interface{} {
	n := len(*l)
	v := (*l)[n-1]
	*l = (*l)[0 : n-1]
	return v
}

func shortCircuitCycle(queue LifecycleQueue, f func(Lifecycle) error) error {
	queues := make(LifecycleQueue, len(queue))
	copy(queues, queue)
	for {
		if queues.Len() == 0 {
			return nil
		}
		pop := heap.Pop(&queues)
		lifecycle := pop.(Lifecycle)
		if err := f(lifecycle); err != nil {
			return err
		}
	}
}

func (l *LifecycleComposite) PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		if err := lifecycle.PostInitClientConfig(node, clientConfig); err != nil {
			return errors.Wrap(err, "Error at PostInitClientConfig")
		}
		return nil
	})
}

func (l *LifecycleComposite) PostSSHDial(node *Node, client *ssh.Client) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		if err := lifecycle.PostSSHDial(node, client); err != nil {
			return errors.Wrap(err, "Error at PostSSHDial")
		}
		return nil
	})
}

func (l *LifecycleComposite) PostNewSession(node *Node, session *ssh.Session) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		if err := lifecycle.PostNewSession(node, session); err != nil {
			return errors.Wrap(err, "Error at PostNewSession")
		}
		return nil
	})
}

func (l *LifecycleComposite) PostShell(node *Node, stdin io.WriteCloser) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		if err := lifecycle.PostShell(node, stdin); err != nil && err != ErrorInterrupt {
			return errors.Wrap(err, "Error at PostShell")
		}
		return nil
	})
}

func (l *LifecycleComposite) OnStdout(node *Node, line []byte) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		if err := lifecycle.OnStdout(node, line); err != nil {
			return errors.Wrap(err, "Error at OnStdout")
		}
		return nil
	})
}

func (l *LifecycleComposite) OnStderr(node *Node, line []byte) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		if err := lifecycle.OnStderr(node, line); err != nil {
			return errors.Wrap(err, "Error at OnStderr")
		}
		return nil
	})
}

func (l *LifecycleComposite) PostSessionWait(node *Node) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		if err := lifecycle.PostSessionWait(node); err != nil {
			return errors.Wrap(err, "Error at OnStderr")
		}
		return nil
	})
}

// CommonLifecycle is used to build Lifecycle quickly
type CommonLifecycle struct {
	Name                     string
	PriorityFunc             func() int
	PostInitClientConfigFunc func(node *Node, clientConfig *ssh.ClientConfig) error
	PostSSHDialFunc          func(node *Node, client *ssh.Client) error
	PostNewSessionFunc       func(node *Node, session *ssh.Session) error
	PostShellFunc            func(node *Node, stdin io.WriteCloser) error
	OnStdoutFunc             func(node *Node, line []byte) error
	OnStderrFunc             func(node *Node, line []byte) error
	PostSessionWaitFunc      func(node *Node) error
}

var _ Lifecycle = new(CommonLifecycle)

func (d *CommonLifecycle) Priority() int {
	if d.PriorityFunc != nil {
		return d.PriorityFunc()
	}
	return 0
}

func (d *CommonLifecycle) PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error {
	if d.PostInitClientConfigFunc != nil {
		return d.PostInitClientConfigFunc(node, clientConfig)
	}
	return nil
}

func (d *CommonLifecycle) PostSSHDial(node *Node, client *ssh.Client) error {
	if d.PostSSHDialFunc != nil {
		return d.PostSSHDialFunc(node, client)
	}
	return nil
}

func (d *CommonLifecycle) PostNewSession(node *Node, session *ssh.Session) error {
	if d.PostNewSessionFunc != nil {
		return d.PostNewSessionFunc(node, session)
	}
	return nil
}

func (d *CommonLifecycle) PostShell(node *Node, stdin io.WriteCloser) error {
	if d.PostShellFunc != nil {
		return d.PostShellFunc(node, stdin)
	}
	return nil
}

func (d *CommonLifecycle) OnStdout(node *Node, line []byte) error {
	if d.OnStdoutFunc != nil {
		return d.OnStdoutFunc(node, line)
	}
	return nil
}

func (d *CommonLifecycle) OnStderr(node *Node, line []byte) error {
	if d.OnStderrFunc != nil {
		return d.OnStderrFunc(node, line)
	}
	return nil
}

func (d *CommonLifecycle) PostSessionWait(node *Node) error {
	if d.PostSessionWaitFunc != nil {
		return d.PostSessionWaitFunc(node)
	}
	return nil
}
