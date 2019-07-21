package sshw

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"regexp"
	"sync"
	"time"
)

func init() {
	mutex := new(sync.Mutex)
	lifecycleCallback := &LifecycleCallback{
		Mutex: mutex,
	}
	RegisterLifecycle(&CommonLifecycle{
		Name:          "callback",
		PostShellFunc: lifecycleCallback.PostShell,
		OnStdoutFunc:  lifecycleCallback.OnStdout,
	})
}

type LifecycleCallback struct {
	IsError bool
	Index   int
	Mutex   *sync.Mutex
	Cond    *sync.Cond
}

func (l *LifecycleCallback) OnStdout(node *Node, line []byte) error {
	l.Mutex.Lock()
	defer l.Mutex.Unlock()
	if len(node.CallbackShells) == 0 || l.Index == len(node.CallbackShells)-1 {
		return nil
	}
	shell := node.CallbackShells[l.Index]
	pattern := shell.ErrorPattern
	if pattern != "" {
		s := string(line)
		matched, err := regexp.MatchString(pattern, s)
		if err != nil {
			return err
		}
		if matched {
			l.IsError = true
		}
	}
	return nil
}

func (l *LifecycleCallback) PostShell(node *Node, stdin io.WriteCloser) error {
	for i := range node.CallbackShells {
		l.Mutex.Lock()
		if l.IsError {
			l.Mutex.Unlock()
			return ErrorInterrupt
		}
		l.Mutex.Unlock()
		shell := node.CallbackShells[i]
		time.Sleep(shell.Delay * time.Millisecond)
		// Copy Shell
		if shell.CpShell.Src != "" {
			err := copyFile(shell.CpShell.Src, shell.CpShell.Tgt, stdin)
			if err != nil {
				return err
			}
		} else {
			// Cmd Shell
			_, _ = stdin.Write([]byte(shell.Cmd + "\r"))
		}

		l.Mutex.Lock()
		l.Index = i
		l.Mutex.Unlock()
		if shell.ErrorPattern != "" {
			time.Sleep(1000 * time.Millisecond)
		}
	}
	return nil
}

func naiveRealpath(p string) string {
	if p[0] == '~' {
		u, _ := user.Current()
		return path.Join(u.HomeDir, string(p[2:]))
	}
	return p
}

func copyFile(filePath, destinationPath string, stdin io.WriteCloser) error {
	realfilePath := naiveRealpath(filePath)
	f, err := os.Open(realfilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, _ := f.Stat()
	perm := info.Mode().Perm()
	bytes, _ := ioutil.ReadAll(f)
	s := "echo -n -e '" + naiveHex(bytes) + "' > " + destinationPath +
		" && " + fmt.Sprintf("chmod %#o %v ", perm, destinationPath) + "\r"
	_, _ = stdin.Write([]byte(s))
	return nil
}

const hextable = "0123456789abcdef"

func naiveHex(src []byte) string {
	dst := make([]byte, len(src)*4)
	for i, v := range src {
		dst[i*4] = '\\'
		dst[i*4+1] = 'x'
		dst[i*4+2] = hextable[v>>4]
		dst[i*4+3] = hextable[v&0x0f]
	}
	return string(dst)
}
