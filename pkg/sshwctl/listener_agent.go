package sshwctl

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"math"
	"net"
	"os"
	"strings"
)

func init() {
	lifecycleAgent := new(LifecycleAgent)
	RegisterLifecycle(&CommonLifecycle{
		Name:                     "agent",
		PriorityFunc:             lifecycleAgent.Priority,
		PostInitClientConfigFunc: lifecycleAgent.PostInitClientConfig,
		PostSSHDialFunc:          lifecycleAgent.PostSSHDial,
		PostNewSessionFunc:       lifecycleAgent.PostNewSession,
	})
}

type LifecycleAgent struct {
	agent agent.Agent
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
		if err := agent.ForwardToAgent(client, l.agent); err != nil {
			return err
		}
	}
	return nil
}

func (l *LifecycleAgent) PostNewSession(node *Node, session *ssh.Session) error {
	if l.agent != nil {
		if err := agent.RequestAgentForwarding(session); err != nil && !strings.Contains(err.Error(), "forwarding request denied") {
			return err
		}
	}
	return nil
}
