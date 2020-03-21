package sshwctl

import (
	"github.com/ljun20160606/eventbus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"math"
	"net"
	"os"
	"strings"
)

const (
	KeyAgent = "agent"
)

func init() {
	_ = bus.Subscribe(PostInitClientConfig, AgentPostInitClientConfig, eventbus.WithOrder(math.MaxInt32))
	_ = bus.Subscribe(PostSSHDial, AgentPostSSHDial, eventbus.WithOrder(math.MaxInt32))
	_ = bus.Subscribe(PostNewSession, AgentPostNewSession, eventbus.WithOrder(math.MaxInt32))
}

func AgentPostInitClientConfig(ctx *EventContext, clientConfig *ssh.ClientConfig) {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		client := agent.NewClient(sshAgent)
		agentAuthMethod := ssh.PublicKeysCallback(client.Signers)

		clientConfig.Auth = append(clientConfig.Auth, agentAuthMethod)
		ctx.Put(KeyAgent, client)
	}
}

func AgentPostSSHDial(ctx *EventContext, client *ssh.Client) error {
	if c, has := ctx.Get(KeyAgent); has {
		if err := agent.ForwardToAgent(client, c.(agent.Agent)); err != nil {
			return err
		}
	}
	return nil
}

func AgentPostNewSession(ctx *EventContext, session *ssh.Session) error {
	if _, has := ctx.Get(KeyAgent); has {
		if err := agent.RequestAgentForwarding(session); err != nil &&
			!strings.Contains(err.Error(), "forwarding request denied") {
			return err
		}
	}
	return nil
}
