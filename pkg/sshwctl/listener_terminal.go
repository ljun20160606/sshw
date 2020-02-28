package sshwctl

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"math"
	"os"
	"time"
)

func init() {
	lifecycleTerminal := new(LifecycleTerminal)
	RegisterLifecycle(&CommonLifecycle{
		Name:               "terminal",
		PostNewSessionFunc: lifecycleTerminal.PostNewSession,
		PriorityFunc:       lifecycleTerminal.Priority,
		PostShellFunc:      lifecycleTerminal.PostShell,
	})
}

type LifecycleTerminal struct {
	session       *ssh.Session
	width, height int
	fd            int

	Callback func()
}

func (*LifecycleTerminal) Priority() int {
	return math.MinInt32
}

func (l *LifecycleTerminal) PostNewSession(node *Node, session *ssh.Session) error {
	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return err
	}
	l.Callback = func() {
		terminal.Restore(fd, state)
	}

	w, h, err := terminal.GetSize(fd)
	if err != nil {
		return err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	err = session.RequestPty("xterm", h, w, modes)
	if err != nil {
		return err
	}
	l.session = session
	l.height = h
	l.width = w
	l.fd = fd
	return nil
}

func (l *LifecycleTerminal) PostShell(node *Node, stdin io.WriteCloser) error {
	// interval get terminal size
	// fix resize issue
	go func() {
		var (
			ow = l.width
			oh = l.height
		)
		for {
			cw, ch, err := terminal.GetSize(l.fd)
			if err != nil {
				break
			}

			if cw != ow || ch != oh {
				err = l.session.WindowChange(ch, cw)
				if err != nil {
					break
				}
				ow = cw
				oh = ch
			}
			time.Sleep(time.Second)
		}
	}()
	return nil
}

func (l *LifecycleTerminal) PostSessionWait(node *Node) error {
	if l.Callback != nil {
		l.Callback()
	}
	return nil
}
