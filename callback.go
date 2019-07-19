package sshw

import (
	"golang.org/x/crypto/ssh"
	"io"
	"time"
)

func init() {
	RegisterLifecycle(new(LifecycleCallback))
}

var _ Lifecycle = new(LifecycleCallback)

type LifecycleCallback struct {
}

func (*LifecycleCallback) PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error {
	return nil
}

func (*LifecycleCallback) PostSSHDial(node *Node, client *ssh.Client) error {
	return nil
}

func (*LifecycleCallback) PostNewSession(node *Node, session *ssh.Session) error {
	return nil
}

func (*LifecycleCallback) PostShell(node *Node, stdin io.WriteCloser) error {
	// then callback
	for i := range node.CallbackShells {
		shell := node.CallbackShells[i]
		time.Sleep(shell.Delay * time.Millisecond)
		_, _ = stdin.Write([]byte(shell.Cmd + "\r"))
	}
	return nil
}

func (*LifecycleCallback) Priority() int {
	return 0
}
