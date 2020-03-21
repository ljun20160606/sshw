package sshwctl

import (
	"bufio"
	"github.com/ljun20160606/eventbus"
	"golang.org/x/crypto/ssh"
	"io"
	"math"
	"time"
)

func init() {
	_ = bus.Subscribe(OnStdout, IOOnStdout, eventbus.WithOrder(math.MinInt32))
	_ = bus.Subscribe(OnStderr, IOOnStderr, eventbus.WithOrder(math.MinInt32))
}

func IOOnStdout(ctx *EventContext, bytes []byte) error {
	node := ctx.Node
	_, err := node.stdout().Write(bytes)
	return err
}

func IOOnStderr(ctx *EventContext, bytes []byte) error {
	node := ctx.Node
	_, err := node.stderr().Write(bytes)
	return err
}

func readLine(node *Node, session *ssh.Session, getReader func() (io.Reader, error), lineSolver func(line []byte) error) error {
	r, err := getReader()
	if err != nil {
		return err
	}

	byteChan := make(chan byte)
	go func() {
		reader := bufio.NewReader(r)
		for {
			b, err := reader.ReadByte()
			if err != nil {
				if err != io.EOF {
					node.Error(err)
				}
				_ = session.Close()
				return
			}
			byteChan <- b
		}
	}()

	go func() {
		var buf []byte
		for {
			select {
			case b := <-byteChan:
				buf = append(buf, b)
			case <-time.After(10 * time.Millisecond):
				if len(buf) == 0 {
					continue
				}

				if err := lineSolver(buf); err != nil {
					node.Error(err)
					_ = session.Close()
					return
				}
				buf = buf[:0]
			}
		}
	}()
	return nil
}
