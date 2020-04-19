package sshwctl

import (
	"bytes"
	"context"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/agent"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
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
	// -----local
	// run pre commands
	ExecsPre() error
	CanConnect() bool
	// terminal makeRaw and store width height
	InitTerminal() error
	RecoverTerminal()
	// run post commands
	ExecsPost() error
	WatchWindowChange(windowChange func(ch, cw int) error)

	// -----local or remote
	Connect() error
	Scp() error
	Shell() error
	Close() error

	// -----remote
	GetClient() *ssh.Client
	SetClient(client *ssh.Client)
	// send keepalive to ssh server
	Ping() error
}

func (c *localClient) Ping() error {
	if c.client == nil {
		return errors.New("need init ssh.client")
	}
	_, _, err := c.client.SendRequest("keepalive@openssh.com", false, nil)
	return err
}

func NewClient(node *Node) Client {
	return newClient(node)
}

type localClient struct {
	clientConfig *ssh.ClientConfig
	node         *Node
	agent        agent.Agent
	client       *ssh.Client
	eventContext *EventContext
	ctx          context.Context
	cancelFunc   context.CancelFunc
}

func (c *localClient) CanConnect() bool {
	return len(c.node.ExecsPre) == 0 || c.node.Host != ""
}

func (c *localClient) GetClient() *ssh.Client {
	return c.client
}

func (c *localClient) SetClient(client *ssh.Client) {
	c.client = client
}

func newClient(node *Node) *localClient {
	config := &ssh.ClientConfig{
		User:            node.user(),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 10,
	}

	eventContext := NewEventContext(node)

	if err := bus.Publish(PostInitClientConfig, eventContext, config); err != nil {
		node.Error(err)
	}

	config.SetDefaults()
	config.Ciphers = append(config.Ciphers, DefaultCiphers...)

	return &localClient{
		clientConfig: config,
		eventContext: eventContext,
		node:         node,
	}
}

func (c *localClient) Dial() (*ssh.Client, error) {
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

func (c *localClient) dial() (*ssh.Client, error) {
	client, err := ssh.Dial("tcp", c.node.addr(), c.clientConfig)
	if err != nil {
		msg := err.Error()
		// use terminal password retry
		if strings.Contains(msg, "no supported methods remain") && !strings.Contains(msg, "password") {
			c.node.Print(fmt.Sprintf("%s@%s's password:", c.clientConfig.User, c.node.Host))
			var b []byte
			// todo fd
			b, err = terminal.ReadPassword(0)
			if err == nil {
				p := string(b)
				if p != "" {
					c.clientConfig.Auth = append(c.clientConfig.Auth, ssh.Password(p))
				}
				c.node.Println("")
				client, err = ssh.Dial("tcp", c.node.addr(), c.clientConfig)
			}
		}
		return nil, err
	}
	return client, nil
}

func (c *localClient) dialByChannel(client *ssh.Client) (*ssh.Client, error) {
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

func (c *localClient) ExecsPre() error {
	if hasVar, err := execs(c.node.ExecsPre, c.node.stdin(), c.node.stdout()); err != nil {
		return err
	} else if hasVar {
		if err := PrepareConfig(c.node); err != nil {
			return err
		}
	}
	return nil
}

func (c *localClient) Connect() error {
	client, err := c.Dial()
	if err != nil {
		return err
	}

	c.client = client

	c.node.Println(fmt.Sprintf("connect server ssh -p %d %s@%s version: %s\n", c.node.port(), c.node.user(), c.node.Host, string(client.ServerVersion())))

	if err := bus.Publish(PostSSHDial, c.eventContext, c.client); err != nil {
		return err
	}

	c.ctx, c.cancelFunc = context.WithCancel(context.Background())

	// send keepalive
	go func() {
		for {
			time.Sleep(time.Second * 5)
			select {
			case <-c.ctx.Done():
				return
			default:
				if err := c.Ping(); err != nil && strings.Contains(err.Error(), "use of closed network") {
					return
				}
			}
		}
	}()

	return nil
}

func (c *localClient) InitTerminal() error {
	fd := int(os.Stdin.Fd())
	if state, err := terminal.MakeRaw(fd); err != nil {
		return err
	} else {
		c.node.State = state
	}

	w, h, err := terminal.GetSize(fd)
	if err != nil {
		return err
	}
	c.node.Width = w
	c.node.Height = h
	return nil
}

func (c *localClient) Scp() error {
	if c.client == nil {
		return errors.New("scp must start client")
	}

	for i := range c.node.Scps {
		nodeCp := c.node.Scps[i]
		if err := c.scp(nodeCp); err != nil {
			return err
		}
	}
	return nil
}

// send terminal request in session
func (c *localClient) xterm(session *ssh.Session) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm", c.node.Height, c.node.Width, modes); err != nil {
		return err
	}
	return nil
}

func (c *localClient) ChangeWindow(session *ssh.Session, ch, cw int) error {
	return session.WindowChange(ch, cw)
}

func (c *localClient) WatchWindowChange(windowChange func(ch, cw int) error) {
	go func() {
		// interval get terminal size
		// fix resize issue
		var (
			ow = c.node.Width
			oh = c.node.Height
		)
		for {
			cw, ch, err := terminal.GetSize(int(os.Stdin.Fd()))
			if err != nil {
				break
			}

			if cw != ow || ch != oh {
				if err := windowChange(ch, cw); err != nil {
					break
				}
				ow = cw
				oh = ch
			}
			time.Sleep(time.Second)
		}
	}()
}

func (c *localClient) RecoverTerminal() {
	_ = terminal.Restore(int(os.Stdin.Fd()), c.node.State)
}

func (c *localClient) Shell() error {
	if c.client == nil {
		return errors.New("shell must start client")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer func() {
		_ = session.Close()
	}()

	c.node.Session = session

	if err := c.xterm(session); err != nil {
		return err
	}

	if err := bus.Publish(PostNewSession, c.eventContext, session); err != nil {
		return err
	}

	// stdin, must be here, get it before session start
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return err
	}

	l := NewCallbackInfo()
	c.eventContext.Put(KeyCallback, l)

	// stdout
	if err := readLine(c.node, session, session.StdoutPipe, func(line []byte) error {
		return bus.Publish(OnStdout, c.eventContext, line)
	}); err != nil {
		return errors.Wrap(err, "stdout")
	}

	// stderr
	if err := readLine(c.node, session, session.StderrPipe, func(line []byte) error {
		return bus.Publish(OnStderr, c.eventContext, line)
	}); err != nil {
		return errors.Wrap(err, "stderr")
	}

	// shell
	if err := session.Shell(); err != nil {
		return err
	}

	if err := bus.Publish(PostShell, c.eventContext, stdinPipe); err != nil {
		return err
	}

	// change stdin to user
	go func() {
		if _, err := io.Copy(stdinPipe, c.node.stdin()); err != nil && err != io.EOF {
			c.node.Error(errors.WithMessage(err, "read from stdin"))
		}
	}()

	if err := session.Wait(); err != nil {
		return errors.WithMessage(err, "session wait")
	}
	return nil
}

func (c *localClient) ExecsPost() error {
	if _, err := execs(c.node.ExecsStop, c.node.stdin(), c.node.stdout()); err != nil {
		return err
	}
	return nil
}

// like shell scp
// cp local file into server
func (c *localClient) scp(cp *NodeCp) error {
	session, err := c.client.NewSession()
	if err != nil {
		return errors.New("Failed to create session: " + err.Error())
	}
	realfilePath := cp.Src
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

		// show processing
		c.node.stdout().Write([]byte("File Size: " + humanize.Bytes(uint64(info.Size())) + "\n"))
		scpCounter := NewWriteCounter()
		scpCounter.W = c.node.stdout()
		scpCounter.ProgressTemplate = "Uploading"
		_, err = io.Copy(w, io.TeeReader(f, scpCounter))
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
		c.node.Println("scp " + cp.Src + " remote:" + cp.Tgt)
		defer wg.Done()
		if err := session.Run(fmt.Sprintf("%s -qt %s", "scp", remotePath)); err != nil {
			errCh <- err
			return
		}
	}()

	if cp.Timeout == 0 {
		cp.Timeout = 60
	}

	if waitTimeout(&wg, time.Duration(cp.Timeout)*time.Second) {
		return errors.New(strconv.Itoa(int(cp.Timeout)) + "s timeout when upload files")
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

func (c *localClient) Close() error {
	if c.client == nil {
		return nil
	}
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	return c.client.Close()
}

var (
	homeDir, _  = os.UserHomeDir()
	dotSSHIdRsa = ".ssh/id_rsa"
	userIdRsa   = path.Join(homeDir, dotSSHIdRsa)
)

func UserIdRsaIsNotExist() bool {
	if _, err := os.Stat(userIdRsa); err != nil {
		return os.IsNotExist(err)
	}
	return false
}

// auto ssh-add .ssh/id_rsa
func AutoSSHAgent() error {
	currentShell := Shell()
	showListCmd := exec.Command(currentShell, "-c", "ssh-add -l")
	showOut, err := showListCmd.Output()
	// stop if ssh-add fail
	if err != nil {
		s := string(showOut)
		// if contains below message, means ssh-add is empty
		if !strings.Contains(s, "The agent has no identities") {
			return errors.WithMessage(err, s)
		}
	}
	if !bytes.Contains(showOut, []byte(dotSSHIdRsa)) {
		addCmd := exec.Command(currentShell, "-c", "ssh-add")
		addCmd.Stdout = os.Stdout
		addCmd.Stdin = os.Stdin
		addCmd.Stderr = os.Stderr
		_ = addCmd.Run()
	}
	return nil
}

func Shell() string {
	currentShell := os.Getenv("SHELL")
	if currentShell == "" {
		return "/bin/sh"
	}
	return currentShell
}

// execute command
// set output into env, key is var
// if has var return true
func execs(execs []*NodeExec, stdin io.Reader, stdout io.Writer) (bool, error) {
	var hasVar bool
	currentShell := Shell()
	for i := range execs {
		nodeExec := execs[i]
		cmdStr := nodeExec.Cmd
		command := exec.Command(currentShell, "-c", cmdStr)
		var buffer *bytes.Buffer
		if nodeExec.Var == "" {
			command.Stdout = stdout
		} else {
			hasVar = true
			buffer = bytes.NewBuffer(nil)
			command.Stdout = io.MultiWriter(stdout, buffer)
		}
		command.Stderr = stdout
		command.Stdin = stdin
		_, _ = io.WriteString(stdout, cmdStr+"\n")
		if err := command.Run(); err != nil {
			return hasVar, err
		}
		if buffer != nil {
			// output always contains '\n'
			echo := buffer.String()
			trimEcho := strings.TrimRight(echo, "\n")
			if err := os.Setenv(nodeExec.Var, trimEcho); err != nil {
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
