package sshw

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io"
	"math"
	"net"
	"os"
)

func init() {
	RegisterLifecycle(new(LifecycleAgent))
}

var _ Lifecycle = new(LifecycleAgent)

type LifecycleAgent struct {
	agent agent.Agent
}

func (l *LifecycleAgent) PostShell(node *Node, stdin io.WriteCloser) error {
	return nil
}

func (l *LifecycleAgent) Priority() int {
	return math.MaxInt32
}

func (l *LifecycleAgent) PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		client := agent.NewClient(sshAgent)
		agentAuthMethod := ssh.PublicKeysCallback(client.Signers)

		clientConfig.Auth = append(clientConfig.Auth, agentAuthMethod)
		l.agent = client
	}

	return nil
}

func (l *LifecycleAgent) PostSSHDial(node *Node, client *ssh.Client) error {
	if l.agent != nil {
		err := agent.ForwardToAgent(client, l.agent)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *LifecycleAgent) PostNewSession(node *Node, session *ssh.Session) error {
	if l.agent != nil {
		err := agent.RequestAgentForwarding(session)
		if err != nil {
			return err
		}
	}
	return nil
}
