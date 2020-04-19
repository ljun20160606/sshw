package sshwctl

import (
	"context"
	"io"
	"os"
	"syscall"
	"time"
)

type WriterFunc func(p []byte) (n int, err error)

func (w WriterFunc) Write(p []byte) (n int, err error) {
	return w(p)
}

type ReaderFunc func(p []byte) (n int, err error)

func (r ReaderFunc) Read(p []byte) (n int, err error) {
	return r(p)
}

func NewNonblockReader(r *os.File) (*NonblockReader, error){
	if err := syscall.SetNonblock(int(r.Fd()), true); err != nil {
		return nil, err
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &NonblockReader{
		Ctx:        ctx,
		CancelFunc: cancelFunc,
		R:          r,
	}, nil
}

type NonblockReader struct {
	Ctx context.Context
	CancelFunc context.CancelFunc
	R   *os.File
}

func (n2 *NonblockReader) Read(p []byte) (n int, err error) {
	for {
		select {
		case <-n2.Ctx.Done():
			return 0, io.EOF
		default:
			if n, err := n2.R.Read(p); err != nil {
				if IsEAGAIN(err) {
					time.Sleep(time.Millisecond * 15)
					continue
				}
			} else {
				return n, err
			}
		}
	}
}

func IsEAGAIN(err error) bool {
	if err == nil {
		return false
	}
	if pathErr, ok := err.(*os.PathError); ok {
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			if errno == syscall.EAGAIN {
				return true
			}
		}
	}
	return false
}

func (n2 *NonblockReader) Close() error {
	n2.CancelFunc()
	if err := syscall.SetNonblock(int(n2.R.Fd()), true); err != nil {
		return err
	}
	return nil
}
