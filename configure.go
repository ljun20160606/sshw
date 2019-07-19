package sshw

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

	Priority() int
}

var lifecycleComposite = new(LifecycleComposite)

func RegisterLifecycle(lifecycle Lifecycle) {
	heap.Push(&lifecycleComposite.queue, lifecycle)
}

var _ Lifecycle = new(LifecycleComposite)

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
		err := f(lifecycle)
		if err != nil {
			return err
		}
	}
}

func (l *LifecycleComposite) PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		err := lifecycle.PostInitClientConfig(node, clientConfig)
		if err != nil {
			return errors.Wrap(err, "Error at PostInitClientConfig")
		}
		return nil
	})
}

func (l *LifecycleComposite) PostSSHDial(node *Node, client *ssh.Client) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		err := lifecycle.PostSSHDial(node, client)
		if err != nil {
			return errors.Wrap(err, "Error at PostSSHDial")
		}
		return nil
	})
}

func (l *LifecycleComposite) PostNewSession(node *Node, session *ssh.Session) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		err := lifecycle.PostNewSession(node, session)
		if err != nil {
			return errors.Wrap(err, "Error at PostNewSession")
		}
		return nil
	})
}

func (l *LifecycleComposite) PostShell(node *Node, stdin io.WriteCloser) error {
	return shortCircuitCycle(l.queue, func(lifecycle Lifecycle) error {
		err := lifecycle.PostShell(node, stdin)
		if err != nil {
			return errors.Wrap(err, "Error at PostShell")
		}
		return nil
	})
}
