package multiplex

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	PathStdin    = "stdin"
	PathStdout   = "stdout"
	PathStderr   = "stderr"
	PathSession  = "session"
	PathTerminal = PathSession + "/terminal"
	PathScp      = PathSession + "/scp"
)

type StdConn struct {
	Num    int64
	Stdin  io.ReadCloser
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

type ChangeWindowRequest struct {
	Width  int
	Height int
}

// create a ssh session
type ClientRequest struct {
	// ssh config
	Node *sshwctl.Node
}

// client and server standard response
type PlainResult struct {
	// if code is -1, return error message
	Message string
	// -1 fail, 0 success
	Code int
	// extension data
	Data json.RawMessage
}

type MasterHandler struct {
	// { [node.string]: [sshwClient] }
	clientMap *TimerMap
}

func NewMasterHandler() Handler {
	timerMap := NewTimerMap()
	return &MasterHandler{
		clientMap: timerMap,
	}
}

// show metric data
func (m *MasterHandler) Metric() {
	fmt.Println("clientMap:", m.clientMap.Size())
}

func (m *MasterHandler) GetFd(w ResponseWriter) (*StdConn, error) {
	files, err := Get(w.Conn().(*net.UnixConn), 3, []string{PathStdin, PathStdout, PathStderr})
	if err != nil {
		return nil, err
	}
	return &StdConn{
		Stdin:  files[0],
		Stdout: files[1],
		Stderr: files[2],
	}, nil
}

// serve a sshwctl request
func (m *MasterHandler) Serve(w ResponseWriter, req *Request) {
	// load a client, and create session
	if strings.HasPrefix(req.Path, PathSession) {
		parsedClientRequest := &ClientRequest{}
		_ = json.Unmarshal(req.Body, parsedClientRequest)
		node := parsedClientRequest.Node
		stdConn, err := m.GetFd(w)
		if err != nil {
			m.Fail(w, err)
			m.CloseConn(w)
			return
		}
		node.Stdin = stdConn.Stdin
		node.Stdout = stdConn.Stdout
		node.Stderr = stdConn.Stderr

		client, err := m.NewClient(node)
		if err != nil {
			m.Fail(w, err)
			m.CloseConn(w)
			return
		}
		if strings.HasPrefix(req.Path, PathScp) {
			m.Process(w, stdConn, client.Scp)
			return
		}
		if strings.HasPrefix(req.Path, PathTerminal) {
			name := node.String()
			m.clientMap.IncrRef(name)
			// watch window change
			go func() {
				for {
					window := &ChangeWindowRequest{}
					if err := req.R.Read(window); err != nil {
						return
					}
					if node.Session != nil {
						if err := node.Session.WindowChange(window.Height, window.Width); err != nil {
							return
						}
					}
				}
			}()
			m.Process(w, stdConn, client.Shell)
			m.clientMap.Done(name)
			m.Metric()
			return
		}
	}
	m.CloseConn(w)
}

func (m *MasterHandler) NewClient(node *sshwctl.Node) (sshwctl.Client, error) {
	name := node.String()
	newClient := sshwctl.NewClient(node)
	if !newClient.CanConnect() {
		return nil, errors.New("server: should run local")
	}
	if client, ok := m.GetClient(name); !ok {
		// first auth may need people to solve
		if err := newClient.Connect(); err != nil {
			return nil, err
		}
		m.PutClient(name, newClient)
	} else if err := client.Ping(); err != nil {
		_ = client.Close()
		if err := newClient.Connect(); err != nil {
			return nil, err
		}
		m.PutClient(name, newClient)
	} else {
		newClient.SetClient(client.GetClient())
	}
	return newClient, nil
}

func (m *MasterHandler) GetClient(name string) (sshwctl.Client, bool) {
	load, b := m.clientMap.Load(name)
	if b {
		return load.(sshwctl.Client), true
	}
	return nil, false
}

func (m *MasterHandler) PutClient(name string, client sshwctl.Client) {
	m.clientMap.Insert(name, client, func(key string, value interface{}) {
		client := value.(sshwctl.Client)
		_ = client.Close()
	})
}

func (m *MasterHandler) Process(w ResponseWriter, stdConn *StdConn, callables ...func() error) {
	for i := range callables {
		f := callables[i]
		if err := f(); err != nil {
			m.Fail(w, err)
			m.CloseConn(w)
			return
		}
	}
	m.Success(w, "ok")
	m.CloseConn(w)
}

// close serve conn
func (m *MasterHandler) CloseConn(w ResponseWriter) {
	_ = w.Conn().Close()
}

func (m *MasterHandler) Success(w ResponseWriter, data interface{}) {
	bytes, _ := json.Marshal(data)
	marshal, _ := json.Marshal(PlainResult{
		Data: bytes,
		Code: 0,
	})
	_ = w.Write(&Response{Body: marshal})
}

func (m *MasterHandler) Fail(w ResponseWriter, err error) {
	marshal, _ := json.Marshal(PlainResult{
		Message: err.Error(),
		Code:    -1,
	})
	_ = w.Write(&Response{Body: marshal})
}

// to control recycle of resource
// open a daemon goroutine, poll map periodically and delete data that is expired and zeroRef
type TimerMap struct {
	Kv       sync.Map
	Interval time.Duration
	Timeout  time.Duration
}

func NewTimerMap() *TimerMap {
	t := &TimerMap{
		Interval: time.Second,
		Timeout:  time.Minute * 10,
	}
	go t.Daemon()
	return t
}

func (t *TimerMap) IncrRef(key string) {
	load, ok := t.Kv.Load(key)
	if ok {
		timerEntry := load.(*TimerEntry)
		timerEntry.IncrRef()
	}
}

func (t *TimerMap) Done(key string) {
	load, ok := t.Kv.Load(key)
	if ok {
		timerEntry := load.(*TimerEntry)
		timerEntry.Done()
	}
}

func (t *TimerMap) Insert(key string, value interface{}, callback func(key string, value interface{})) {
	item := &TimerEntry{Key: key, Value: value, ExpiredTime: time.Now().Add(t.Timeout), Callback: callback}
	t.Kv.Store(key, item)
}

func (t *TimerMap) Load(key string) (interface{}, bool) {
	load, ok := t.Kv.Load(key)
	if ok {
		timerEntry := load.(*TimerEntry)
		timerEntry.ExpiredTime = time.Now().Add(t.Timeout)
		return timerEntry.Value, true
	}
	return nil, false
}

func (t *TimerMap) Delete(key string) {
	t.Kv.Delete(key)
}

func (t *TimerMap) Size() int {
	var length int
	t.Kv.Range(func(key, value interface{}) bool {
		length++
		return true
	})
	return length
}

func (t *TimerMap) Daemon() {
	for {
		time.Sleep(t.Interval)
		var expiredEntry []*TimerEntry
		t.Kv.Range(func(key, value interface{}) bool {
			timerEntry := value.(*TimerEntry)
			if !timerEntry.IsExpired() {
				return true
			}
			if timerEntry.ZeroRef() {
				expiredEntry = append(expiredEntry, timerEntry)
				return true
			}
			timerEntry.ExpiredTime = time.Now().Add(t.Timeout)
			return true
		})
		for i := range expiredEntry {
			timerEntry := expiredEntry[i]
			t.Kv.Delete(timerEntry)
			timerEntry.Callback(timerEntry.Key, timerEntry.Value)
		}
	}
}

type TimerEntry struct {
	Key         string
	Value       interface{}
	ExpiredTime time.Time
	Callback    func(key string, value interface{})
	ReferNum    int64
}

func (t *TimerEntry) IsExpired() bool {
	return time.Now().After(t.ExpiredTime)
}

func (t *TimerEntry) ZeroRef() bool {
	return atomic.LoadInt64(&t.ReferNum) <= 0
}

func (t *TimerEntry) IncrRef() {
	atomic.AddInt64(&t.ReferNum, 1)
}

func (t *TimerEntry) Done() {
	atomic.AddInt64(&t.ReferNum, -1)
}
