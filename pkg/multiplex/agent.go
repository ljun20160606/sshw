package multiplex

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"io"
	"net"
	"os"
	"sync"
)

func IsRunning() bool {
	dial, err := net.Dial("unix", SocketPath)
	if err == nil {
		_ = dial.Close()
		return true
	}
	return false
}

func ExecNode(node *sshwctl.Node) error {
	client := sshwctl.NewClient(node)
	// local
	if err := client.ExecsPre(); err != nil {
		return err
	}
	// agent
	agent := Agent{Node: node, Client: client}
	num, err := agent.GetNum()
	if err != nil {
		return err
	}
	errCh := agent.Forward(num)
	defer close(errCh)
	if err := client.InitTerminal(); err != nil {
		return err
	}
	if err := agent.OpenSession(num); err != nil {
		fmt.Println("[sshw]", err.Error())
	}
	// local
	if err := client.ExecsPost(); err != nil {
		return err
	}
	return nil
}

type Agent struct {
	Node   *sshwctl.Node
	Client sshwctl.Client
}

// get a session number
func (c *Agent) GetNum() (int64, error) {
	var num int64
	if err := c.rpcGet(PathCreateConn, &num); err != nil {
		return 0, err
	}
	return num, nil
}

// forward stdout, stderr, stdin to master
func (c *Agent) Forward(num int64) chan error {
	errCh := make(chan error)
	group := &sync.WaitGroup{}
	group.Add(3)
	go c.connOut(group, PathStdout, num, os.Stdout, errCh)
	go c.connOut(group, PathStderr, num, os.Stderr, errCh)
	go c.connIn(group, PathStdin, num, errCh)
	group.Wait()
	return errCh
}

// open a new session
func (c *Agent) OpenSession(num int64) error {
	conn, _ := net.Dial("unix", SocketPath)
	writer := NewJsonProtoWriter(conn)
	clientReq := &ClientRequest{
		Num:  num,
		Node: c.Node,
	}
	body, _ := json.Marshal(clientReq)
	_ = writer.Write(&Request{
		Path: PathSession,
		Body: body,
	})

	// watch window change
	c.Client.WatchWindowChange(func(ch, cw int) error {
		request := ChangeWindowRequest{
			Width:  cw,
			Height: ch,
		}
		if err := writer.Write(request); err != nil {
			return err
		}
		return nil
	})

	// read error message
	reader := NewJsonProtoReader(conn)
	resp := new(Response)
	_ = reader.Read(resp)
	p := &PlainResult{}
	_ = json.Unmarshal(resp.Body, p)
	if p.Code != 0 {
		return errors.New(p.Message)
	}
	return nil
}

func (c *Agent) rpcGet(path string, result interface{}) error {
	conn, _ := net.Dial("unix", SocketPath)
	w := NewJsonProtoWriter(conn)
	_ = w.Write(&Request{
		Path: path,
	})
	r := NewJsonProtoReader(conn)
	resp := new(Response)
	_ = r.Read(resp)
	p := &PlainResult{}
	_ = json.Unmarshal(resp.Body, p)
	if p.Code != 0 {
		return errors.New(p.Message)
	}
	return json.Unmarshal(p.Data, result)
}

func (c *Agent) connOut(group *sync.WaitGroup, path string, num int64, writer io.Writer, errCh chan error) {
	conn, _ := net.Dial("unix", SocketPath)
	w := NewJsonProtoWriter(conn)
	bytes, _ := json.Marshal(num)
	_ = w.Write(&Request{
		Path: path,
		Body: bytes,
	})
	group.Done()
	_, err := io.Copy(writer, conn)
	select {
	case errCh <- err:
	default:
	}
}

func (c *Agent) connIn(group *sync.WaitGroup, path string, num int64, errCh chan error) {
	conn, _ := net.Dial("unix", SocketPath)
	w := NewJsonProtoWriter(conn)
	bytes, _ := json.Marshal(num)
	_ = w.Write(&Request{
		Path: path,
		Body: bytes,
	})
	group.Done()
	_, err := io.Copy(conn, os.Stdin)
	select {
	case errCh <- err:
	default:
	}
}
