package sshwctl

import (
	"container/heap"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLifecycleQueue(t *testing.T) {
	ast := assert.New(t)
	chanints := make(chan int)
	composite := new(LifecycleComposite)
	for i := 0; i < 10; i++ {
		var t = i
		heap.Push(&composite.queue, &CommonLifecycle{
			PriorityFunc: func() int {
				return t
			},
			OnStderrFunc: func(node *Node, line []byte) error {
				chanints <- t
				return nil
			},
		})
	}

	go func() {
		_ = composite.OnStderr(nil, nil)
	}()

	for i := 9; i >= 0; i-- {
		ast.Equal(i, <-chanints)
	}
}
