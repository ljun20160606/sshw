package multiplex

import (
	"encoding/json"
	"errors"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"golang.org/x/crypto/ssh"
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

type MasterClient struct {
	LocalClient sshwctl.Client
	Node        *sshwctl.Node
}

func NewClient(node *sshwctl.Node) sshwctl.Client {
	client := sshwctl.NewClient(node)
	return &MasterClient{
		LocalClient: client,
		Node:        node,
	}
}

func (m *MasterClient) ExecsPre() error {
	return m.LocalClient.ExecsPre()
}

func (m *MasterClient) CanConnect() bool {
	return m.LocalClient.CanConnect()
}

func (m *MasterClient) InitTerminal() error {
	return m.LocalClient.InitTerminal()
}

func (m *MasterClient) RecoverTerminal() {
	m.LocalClient.RecoverTerminal()
}

func (m *MasterClient) ExecsPost() error {
	return m.LocalClient.ExecsPost()
}

func (m *MasterClient) WatchWindowChange(windowChange func(ch, cw int) error) {
	return
}

func (m *MasterClient) Connect() error {
	return nil
}

func (m *MasterClient) Scp() error {
	conn, _ := net.Dial("unix", SocketPath)
	writer := NewJsonProtoWriter(conn)
	clientReq := &ClientRequest{
		Node: m.Node,
	}
	body, _ := json.Marshal(clientReq)
	_ = writer.Write(&Request{
		Path: PathScp,
		Body: body,
	})

	if err := Put(conn.(*net.UnixConn), os.Stdin, os.Stdout, os.Stderr); err != nil {
		return err
	}

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

func (m *MasterClient) Shell() error {
	conn, _ := net.Dial("unix", SocketPath)
	writer := NewJsonProtoWriter(conn)
	clientReq := &ClientRequest{
		Node: m.Node,
	}
	body, _ := json.Marshal(clientReq)
	_ = writer.Write(&Request{
		Path: PathTerminal,
		Body: body,
	})

	if err := Put(conn.(*net.UnixConn), os.Stdin, os.Stdout, os.Stderr); err != nil {
		return err
	}

	// watch window change
	m.LocalClient.WatchWindowChange(func(ch, cw int) error {
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

func (m *MasterClient) Close() error {
	return nil
}

func (m *MasterClient) GetClient() *ssh.Client {
	panic("implement me")
}

func (m *MasterClient) SetClient(client *ssh.Client) {
	panic("implement me")
}

func (m *MasterClient) Ping() error {
	panic("implement me")
}
