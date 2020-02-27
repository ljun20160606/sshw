package sshw

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/agent"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
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
	Connect() error
	OpenTerminal() error
	Close() error
}

func NewClient(node *Node) Client {
	return newClient(node)
}

type defaultClient struct {
	clientConfig *ssh.ClientConfig
	node         *Node
	agent        agent.Agent
	client       *ssh.Client
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
			b, err = terminal.ReadPassword(syscall.Stdin)
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

func (c *defaultClient) Connect() error {
	if hasVar, err := execs(c.node.ExecsPre); err != nil {
		return err
	} else if hasVar {
		if err = PrepareConfig(c.node); err != nil {
			return err
		}
	}
	if len(c.node.ExecsPre) != 0 && c.node.Host == "" {
		return errors.New("can not run")
	}

	client, err := c.Dial()
	if err != nil {
		return err
	}

	c.client = client

	l.Infof("connect server ssh -p %d %s@%s version: %s\n", c.node.port(), c.node.user(), c.node.Host, string(client.ServerVersion()))

	return nil
}

func (c *defaultClient) OpenTerminal() error {
	if c.client == nil {
		return errors.New("must start client")
	}

	if err := lifecycleComposite.PostSSHDial(c.node, c.client); err != nil {
		return err
	}

	for i := range c.node.Scps {
		nodeCp := c.node.Scps[i]
		if err := c.Scp(nodeCp); err != nil {
			return err
		}
	}

	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer func() {
		_ = session.Close()
	}()

	if err := lifecycleComposite.PostNewSession(c.node, session); err != nil {
		return err
	}

	// stdin
	stdinPipe, err := session.StdinPipe()
	if err != nil {
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
			time.Sleep(time.Second * 5)
			_, _, _ = c.client.SendRequest("keepalive@openssh.com", false, nil)
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

// like shell scp
// cp local file into server
func (c *defaultClient) Scp(cp *NodeCp) error {
	session, err := c.client.NewSession()
	if err != nil {
		return errors.New("Failed to create session: " + err.Error())
	}
	realfilePath := naiveRealpath(cp.Src)
	f, err := os.Open(realfilePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	info, _ := f.Stat()
	perm := fmt.Sprintf("%04o", info.Mode().Perm())

	remotePath := cp.Tgt
	filename := path.Base(cp.Src)

	wg := sync.WaitGroup{}
	wg.Add(2)

	errCh := make(chan error, 2)

	go func() {
		defer wg.Done()
		w, err := session.StdinPipe()
		if err != nil {
			errCh <- err
			return
		}

		defer func() {
			_ = w.Close()
		}()

		stdout, err := session.StdoutPipe()

		if err != nil {
			errCh <- err
			return
		}

		_, err = fmt.Fprintln(w, "C"+perm, info.Size(), filename)
		if err != nil {
			errCh <- err
			return
		}

		if err = checkResponse(stdout); err != nil {
			errCh <- err
			return
		}

		_, err = io.Copy(w, f)
		if err != nil {
			errCh <- err
			return
		}

		_, err = fmt.Fprint(w, "\x00")
		if err != nil {
			errCh <- err
			return
		}

		if err = checkResponse(stdout); err != nil {
			errCh <- err
			return
		}
	}()

	go func() {
		fmt.Println("scp " + cp.Src + " remote:" + cp.Tgt)
		defer wg.Done()
		if err := session.Run(fmt.Sprintf("%s -qt %s", "scp", remotePath)); err != nil {
			errCh <- err
			return
		}
	}()

	var timeout int64
	if cp.Timeout == 0 {
		timeout = 60
	}

	if waitTimeout(&wg, time.Duration(timeout)*time.Second) {
		return errors.New("timeout when upload files")
	}

	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

func (c *defaultClient) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
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

// Checks the response it reads from the remote, and will return a single error in case
// of failure
func checkResponse(r io.Reader) error {
	response, err := ParseResponse(r)
	if err != nil {
		return err
	}

	if response.IsFailure() {
		return errors.New(response.GetMessage())
	}

	return nil

}
