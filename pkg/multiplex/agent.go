package multiplex

import (
	"bytes"
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

var (
	sigint  = []byte{3}
	sigterm = []byte{4}
)

func (m *masterClient) Scp(_ context.Context) error {
	if len(m.Node.Scps) == 0 {
		return nil
	}
	num, err := GetNum()
	if err != nil {
		return err
	}

	once := sync.Once{}
	done := make(chan struct{})
	defer close(done)
	wrapperConn := Forward(num, func(p []byte) {
		// cancel scp
		if bytes.Equal(p, sigint) || bytes.Equal(p, sigterm) {
			once.Do(func() {
				done <- struct{}{}
			})
		}
	})
	defer wrapperConn.Close()
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

	// wait cancel scp
	go func() {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
			conn, _ := net.Dial("unix", SocketPath)
			writer := NewJsonProtoWriter(conn)
			clientReq := &ClientRequest{
				Num: num,
			}
			body, _ := json.Marshal(clientReq)
			_ = writer.Write(&Request{
				Path: PathCancel,
				Body: body,
			})
		}
	}()

	reader := NewJsonProtoReader(conn)
	return readSuccess(reader)
}

func (m *masterClient) Shell() error {
	num, err := GetNum()
	if err != nil {
		return err
	}
	wrapperConn := Forward(num)
	defer wrapperConn.Close()
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

func Forward(num int64, filters ...func(p []byte)) *WrapperConn {
	wrapperConn := NewWrapperConn()

	group := &sync.WaitGroup{}
	group.Add(3)

	go func() {
		conn := connOut(group, PathStdout, num, os.Stdout, wrapperConn.errCh)
		wrapperConn.Lock()
		wrapperConn.connOut = conn
		wrapperConn.Unlock()
	}()
	go func() {
		conn := connOut(group, PathStderr, num, os.Stderr, wrapperConn.errCh)
		wrapperConn.Lock()
		wrapperConn.connErr = conn
		wrapperConn.Unlock()
	}()
	go func() {
		conn := connIn(wrapperConn.ctx, group, PathStdin, num, wrapperConn.errCh, filters...)
		wrapperConn.Lock()
		wrapperConn.connIn = conn
		wrapperConn.Unlock()
	}()
	group.Wait()

	return wrapperConn
}

func connOut(group *sync.WaitGroup, path string, num int64, writer io.Writer, errCh chan error) net.Conn {
	conn, _ := net.Dial("unix", SocketPath)
	w := NewJsonProtoWriter(conn)
	bytes, _ := json.Marshal(num)
	_ = w.Write(&Request{
		Path: path,
		Body: bytes,
	})
	group.Done()
	go func() {
		_, err := io.Copy(writer, conn)
		select {
		case errCh <- err:
		default:
		}
	}()
	return conn
}

type WrapperConn struct {
	sync.Mutex

	connIn  net.Conn
	connOut net.Conn
	connErr net.Conn

	errCh      chan error
	cancelFunc context.CancelFunc
	ctx        context.Context
}

func (w *WrapperConn) Close() {
	w.connIn.Close()
	w.connOut.Close()
	w.connErr.Close()
	w.cancelFunc()
	close(w.errCh)
}

func NewWrapperConn() *WrapperConn {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &WrapperConn{
		cancelFunc: cancelFunc,
		ctx:        ctx,
		errCh:      make(chan error),
	}
}

func connIn(ctx context.Context, group *sync.WaitGroup, path string, num int64, errCh chan error, filters ...func(p []byte)) net.Conn {
	conn, _ := net.Dial("unix", SocketPath)
	w := NewJsonProtoWriter(conn)
	bytes, _ := json.Marshal(num)
	_ = w.Write(&Request{
		Path: path,
		Body: bytes,
	})
	group.Done()
	go func() {
		_, err := io.Copy(sshwctl.WriterFunc(func(p []byte) (int, error) {
			for _, filter := range filters {
				filter(p)
			}
			return conn.Write(p)
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
	}()
	return conn
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
