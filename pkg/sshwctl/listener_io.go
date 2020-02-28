package sshwctl

import (
	"bufio"
	"golang.org/x/crypto/ssh"
	"io"
	"math"
	"os"
	"time"
)

func init() {
	lifecycleIO := new(LifecycleIO)
	RegisterLifecycle(&CommonLifecycle{
		Name:         "io",
		PriorityFunc: lifecycleIO.Priority,
		OnStdoutFunc: lifecycleIO.OnStdout,
		OnStderrFunc: lifecycleIO.OnStderr,
	})
}

type LifecycleIO struct {
}

func (*LifecycleIO) OnStdout(node *Node, bytes []byte) error {
	_, err := os.Stdout.Write(bytes)
	return err
}

func (*LifecycleIO) OnStderr(node *Node, bytes []byte) error {
	_, err := os.Stderr.Write(bytes)
	return err
}

func (*LifecycleIO) Priority() int {
	return math.MinInt32
}

func readLine(session *ssh.Session, getReader func() (io.Reader, error), lineSolver func(line []byte) error) error {
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
					l.Error(err)
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
					l.Error(err)
					_ = session.Close()
					return
				}
				buf = buf[:0]
			}
		}
	}()
	return nil
}
