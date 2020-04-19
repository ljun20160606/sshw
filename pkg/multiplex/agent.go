package multiplex

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"golang.org/x/crypto/ssh"
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
	num, err := GetNum()
	if err != nil {
		return err
	}
	_ = Forward(num)
	conn, _ := net.Dial("unix", SocketPath)
	writer := NewJsonProtoWriter(conn)
	clientReq := &ClientRequest{
		Num:  num,
		Node: m.Node,
	}
	body, _ := json.Marshal(clientReq)
	_ = writer.Write(&Request{
		Path: PathScp,
		Body: body,
	})

	reader := NewJsonProtoReader(conn)
	return readSuccess(reader)
}

func (m *masterClient) Shell() error {
	num, err := GetNum()
	if err != nil {
		return err
	}
	_ = Forward(num)
	conn, _ := net.Dial("unix", SocketPath)
	writer := NewJsonProtoWriter(conn)
	clientReq := &ClientRequest{
		Num:  num,
		Node: m.Node,
	}
	body, _ := json.Marshal(clientReq)
	_ = writer.Write(&Request{
		Path: PathTerminal,
		Body: body,
	})

	f := func() {
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
	}
	f()
	reader := NewJsonProtoReader(conn)
	return readSuccess(reader)
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

// get a session number
func GetNum() (int64, error) {
	var num int64
	if err := rpcGet(PathCreateConn, &num); err != nil {
		return 0, err
	}
	return num, nil
}

func rpcGet(path string, result interface{}) error {
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

func Forward(num int64) chan error {
	errCh := make(chan error)
	group := &sync.WaitGroup{}
	group.Add(3)
	go connOut(group, PathStdout, num, os.Stdout, errCh)
	go connOut(group, PathStderr, num, os.Stderr, errCh)
	go connIn(group, PathStdin, num, errCh)
	group.Wait()
	return errCh
}

func connOut(group *sync.WaitGroup, path string, num int64, writer io.Writer, errCh chan error) {
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

type WrapperConn struct {
	conn net.Conn
	cancelFunc context.CancelFunc
	ctx context.Context
}

func (w *WrapperConn) Close() error {
	err := w.conn.Close()
	w.cancelFunc()
	return err
}

// 全局唯一
var tempConn *WrapperConn

func setTempConn(c net.Conn) context.Context {
	if tempConn != nil {
		_ = tempConn.Close()
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	tempConn = &WrapperConn{
		conn:       c,
		cancelFunc: cancelFunc,
		ctx:        ctx,
	}
	return ctx
}

func connIn(group *sync.WaitGroup, path string, num int64, errCh chan error) {
	conn, _ := net.Dial("unix", SocketPath)
	ctx := setTempConn(conn)
	w := NewJsonProtoWriter(conn)
	bytes, _ := json.Marshal(num)
	_ = w.Write(&Request{
		Path: path,
		Body: bytes,
	})
	group.Done()
	_, err := io.Copy(sshwctl.WriterFunc(func(p []byte) (int, error) {
		// 由于stdin block模式容易阻塞住上一次read，所以上一次read会阻塞住读到下一次copy的数据，所以需要写到新的conn中
		return tempConn.conn.Write(p)
	}), sshwctl.ReaderFunc(func(p []byte) (int, error) {
		select {
		case <-ctx.Done():
			return 0, io.EOF
		default:
			return os.Stdin.Read(p)
		}
	}))
	select {
	case errCh <- err:
	default:
	}
}

// send fd
func fdWait(conn io.Reader, meanWhile func()) error {
	reader := NewJsonProtoReader(conn)
	var fdPutError error
	for i := 0; i < 3; i++ {
		if err := Put(conn.(*net.UnixConn), os.Stdin); err != nil {
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
