package multiplex

import (
	"encoding/json"
	"errors"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"golang.org/x/crypto/ssh"
	"io"
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

type masterClient struct {
	LocalClient sshwctl.Client
	Node        *sshwctl.Node
}

func NewClient(node *sshwctl.Node) sshwctl.Client {
	client := sshwctl.NewClient(node)
	return &masterClient{
		LocalClient: client,
		Node:        node,
	}
}

func (m *masterClient) ExecsPre() error {
	return m.LocalClient.ExecsPre()
}

func (m *masterClient) CanConnect() bool {
	return m.LocalClient.CanConnect()
}

func (m *masterClient) InitTerminal() error {
	return m.LocalClient.InitTerminal()
}

func (m *masterClient) RecoverTerminal() {
	m.LocalClient.RecoverTerminal()
}

func (m *masterClient) ExecsPost() error {
	return m.LocalClient.ExecsPost()
}

func (m *masterClient) WatchWindowChange(windowChange func(ch, cw int) error) {
	return
}

func (m *masterClient) Connect() error {
	return nil
}

func (m *masterClient) Scp() error {
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

	return fdWait(conn, nil)
}

func (m *masterClient) Shell() error {
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

	return fdWait(conn, func() {
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
	})
}

func (m *masterClient) Close() error {
	return nil
}

func (m *masterClient) GetClient() *ssh.Client {
	panic("implement me")
}

func (m *masterClient) SetClient(client *ssh.Client) {
	panic("implement me")
}

func (m *masterClient) Ping() error {
	panic("implement me")
}

// send fd
func fdWait(conn io.Reader, meanWhile func()) error {
	reader := NewJsonProtoReader(conn)
	var fdPutError error
	for i := 0; i < 3; i++ {
		if err := Put(conn.(*net.UnixConn), os.Stdin, os.Stdout, os.Stderr); err != nil {
			return err
		}

		if fdPutError = readSuccess(reader); fdPutError == nil {
			break
		}
	}
	if meanWhile != nil {
		meanWhile()
	}

	return readSuccess(reader)
}

// read message, if code != 0 return error
func readSuccess(reader ProtoReader) error {
	// read error message
	resp := new(Response)
	_ = reader.Read(resp)
	p := &PlainResult{}
	_ = json.Unmarshal(resp.Body, p)
	if p.Code != 0 {
		return errors.New(p.Message)
	}
	return nil
}
