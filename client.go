package sshw

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/agent"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	DefaultCiphers = []string{
		"aes128-ctr",
		"aes192-ctr",
		"aes256-ctr",
		"aes128-gcm@openssh.com",
		"chacha20-poly1305@openssh.com",
		"arcfour256",
		"arcfour128",
		"arcfour",
		"aes128-cbc",
		"3des-cbc",
		"blowfish-cbc",
		"cast128-cbc",
		"aes192-cbc",
		"aes256-cbc",
	}
)

type Client interface {
	Login() error
}

func NewClient(node *Node) Client {
	return newClient(node)
}

type defaultClient struct {
	clientConfig *ssh.ClientConfig
	node         *Node
	agent        agent.Agent
}

func newClient(node *Node) *defaultClient {
	config := &ssh.ClientConfig{
		User:            node.user(),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 10,
	}

	if err := lifecycleComposite.PostInitClientConfig(node, config); err != nil {
		l.Error(err)
	}

	config.SetDefaults()
	config.Ciphers = append(config.Ciphers, DefaultCiphers...)

	return &defaultClient{
		clientConfig: config,
		node:         node,
	}
}

func (c *defaultClient) Dial() (*ssh.Client, error) {
	jumpNodes := c.node.Jump

	if len(jumpNodes) > 0 {
		jumpNode := jumpNodes[0]
		jumpClient := newClient(jumpNode)
		jumper, err := jumpClient.dial()
		if err != nil {
			return nil, errors.Wrap(err, "jumpNode: "+jumpNode.addr())
		}

		return c.dialByChannel(jumper)
	}
	return c.dial()
}

func (c *defaultClient) dial() (*ssh.Client, error) {
	client, err := ssh.Dial("tcp", c.node.addr(), c.clientConfig)
	if err != nil {
		msg := err.Error()
		// use terminal password retry
		if strings.Contains(msg, "no supported methods remain") && !strings.Contains(msg, "password") {
			fmt.Printf("%s@%s's password:", c.clientConfig.User, c.node.Host)
			var b []byte
			b, err = terminal.ReadPassword(int(syscall.Stdin))
			if err == nil {
				p := string(b)
				if p != "" {
					c.clientConfig.Auth = append(c.clientConfig.Auth, ssh.Password(p))
				}
				fmt.Println()
				client, err = ssh.Dial("tcp", c.node.addr(), c.clientConfig)
			}
		}
		return nil, err
	}
	return client, nil
}

func (c *defaultClient) dialByChannel(client *ssh.Client) (*ssh.Client, error) {
	addr := c.node.addr()
	conn, err := client.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, c.clientConfig)
	if err != nil {
		return nil, err
	}
	client = ssh.NewClient(ncc, chans, reqs)
	return client, nil
}

func (c *defaultClient) Login() error {
	if hasVar, err := execs(c.node.ExecsPre); err != nil {
		return err
	} else if hasVar {
		if err = PrepareConfig(c.node); err != nil {
			return err
		}
	}
	if len(c.node.ExecsPre) != 0 && c.node.Host == "" {
		return nil
	}

	client, err := c.Dial()
	if err != nil {
		return err
	}
	defer client.Close()
	l.Infof("connect server ssh -p %d %s@%s version: %s\n", c.node.port(), c.node.user(), c.node.Host, string(client.ServerVersion()))

	if err = lifecycleComposite.PostSSHDial(c.node, client); err != nil {
		return err
	}

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	if err := lifecycleComposite.PostNewSession(c.node, session); err != nil {
		return err
	}

	// stdout
	if err := readLine(session, session.StdoutPipe, func(line []byte) error {
		return lifecycleComposite.OnStdout(c.node, line)
	}); err != nil {
		return errors.Wrap(err, "stdout")
	}

	// stderr
	if err := readLine(session, session.StderrPipe, func(line []byte) error {
		return lifecycleComposite.OnStderr(c.node, line)
	}); err != nil {
		return errors.Wrap(err, "stderr")
	}

	// stdin
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return err
	}

	// shell
	if err := session.Shell(); err != nil {
		return err
	}

	if err = lifecycleComposite.PostShell(c.node, stdinPipe); err != nil {
		return err
	}

	// change stdin to user
	go func() {
		_, err = io.Copy(stdinPipe, os.Stdin)
		l.Error(err)
		_ = session.Close()
	}()

	// send keepalive
	go func() {
		for {
			time.Sleep(time.Second * 10)
			_, _, _ = client.SendRequest("keepalive@openssh.com", false, nil)
		}
	}()

	_ = session.Wait()
	if err := lifecycleComposite.PostSessionWait(c.node); err != nil {
		return err
	}
	if _, err := execs(c.node.ExecsStop); err != nil {
		return err
	}
	return nil
}

func shell() string {
	currentShell := os.Getenv("SHELL")
	if currentShell == "" {
		return "/bin/sh"
	}
	return currentShell
}

// execute command
// set output into env, key is var
// if has var return true
func execs(execs []*NodeExec) (bool, error) {
	var hasVar bool
	currentShell := shell()
	for i := range execs {
		nodeExec := execs[i]
		cmdStr := nodeExec.Cmd
		command := exec.Command(currentShell, "-c", cmdStr)
		var buffer *bytes.Buffer
		if nodeExec.Var == "" {
			command.Stdout = os.Stdout
		} else {
			hasVar = true
			buffer = bytes.NewBuffer(nil)
			command.Stdout = io.MultiWriter(os.Stdout, buffer)
		}
		command.Stderr = os.Stderr
		command.Stdin = os.Stdin
		_, _ = io.WriteString(os.Stdout, cmdStr+"\n")
		if err := command.Run(); err != nil {
			return hasVar, err
		}
		if buffer != nil {
			if err := os.Setenv(nodeExec.Var, buffer.String()); err != nil {
				return hasVar, err
			}
		}
	}
	return hasVar, nil
}
