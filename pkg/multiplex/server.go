package multiplex

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type Handler interface {
	Serve(w ResponseWriter, req *Request)
}

type ResponseWriter interface {
	Write(resp *Response) error

	Conn() net.Conn
}

type response struct {
	c *conn
}

func (r response) Conn() net.Conn {
	return r.c.rwc
}

func (r response) Write(resp *Response) error {
	return r.c.w.Write(resp)
}

type conn struct {
	rwc net.Conn
	r   ProtoReader
	w   ProtoWriter

	srv *Server
}

// delegate
func (c *conn) serve() {
	req := &Request{}
	_ = c.r.Read(req)
	req.R = c.r
	if c.srv.Handler != nil {
		c.srv.Handler.Serve(&response{c: c}, req)
	}
}

type Server struct {
	// local socket path
	SocketPath string

	Handler Handler
}

func NewServer() *Server {
	m := new(Server)
	return m
}

// when debug stop, can not stop normally, listen retry
var retry int

func (srv *Server) ListenAndServe() error {
	if err := os.MkdirAll(SocketDir, 0755); err != nil {
		return err
	}

	socketPath := srv.socketPath()
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") && strings.HasSuffix(socketPath, "sshw/sshw.socket") {
			if dial, err := net.Dial("unix", SocketPath); err != nil {
				if retry <= 3 {
					_ = os.Remove(socketPath)
					retry++
					return srv.ListenAndServe()
				}
			} else {
				_ = dial.Close()
			}
		}
		return err
	}

	return srv.Serve(listener)
}

func (srv *Server) Serve(l net.Listener) error {
	fmt.Println("sshw master listen " + srv.socketPath())
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				fmt.Println(err)
				continue
			}
			newConn := srv.newConn(conn)
			go func() {
				defer func() {
					if err := recover(); err != nil {
						fmt.Sprintln(err)
					}
				}()
				newConn.serve()
			}()
		}
	}()

	return srv.waitClose(l)
}

func (srv *Server) newConn(rwc net.Conn) *conn {
	return &conn{
		rwc: rwc,
		r:   NewJsonProtoReader(rwc),
		w:   NewJsonProtoWriter(rwc),
		srv: srv,
	}
}

func (srv *Server) waitClose(l net.Listener) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	return l.Close()
}

func (srv *Server) socketPath() string {
	if srv.SocketPath == "" {
		return SocketPath
	}
	return srv.SocketPath
}
