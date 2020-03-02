package multiplex

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"net"
	"os"
)

func IsRunning() bool {
	dial, err := net.Dial("unix", SocketPath)
	if err == nil {
		_ = dial.Close()
		return true
	}
	return false
}

// remote
func ExecNode(node *sshwctl.Node) error {
	client := sshwctl.NewClient(node)
	// local
	if err := client.ExecsPre(); err != nil {
		return err
	}
	if !client.CanConnect() {
		return nil
	}
	// agent
	agent := Agent{Node: node, Client: client}
	num, err := agent.GetNum()
	if err != nil {
		return err
	}
	if err := agent.Forward(num); err != nil {
		return err
	}
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
func (c *Agent) Forward(num int64) error {
	return c.SendFd(num)
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

func (c *Agent) SendFd(num int64) error {
	conn, _ := net.Dial("unix", SocketPath)
	w := NewJsonProtoWriter(conn)
	bytes, _ := json.Marshal(num)
	_ = w.Write(&Request{
		Path: PathStd,
		Body: bytes,
	})
	if err := Put(conn.(*net.UnixConn), os.Stdin, os.Stdout, os.Stderr); err != nil {
		return err
	}
	r := NewJsonProtoReader(conn)
	resp := new(Response)
	_ = r.Read(resp)
	p := &PlainResult{}
	_ = json.Unmarshal(resp.Body, p)
	if p.Code != 0 {
		return errors.New(p.Message)
	}
	return nil
}
