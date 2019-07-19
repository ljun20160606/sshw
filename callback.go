package sshw

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"time"
)

func init() {
	RegisterLifecycle(new(LifecycleCallback))
}

var _ Lifecycle = new(LifecycleCallback)

type LifecycleCallback struct {
}

func (*LifecycleCallback) PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error {
	return nil
}

func (*LifecycleCallback) PostSSHDial(node *Node, client *ssh.Client) error {
	return nil
}

func (*LifecycleCallback) PostNewSession(node *Node, session *ssh.Session) error {
	return nil
}

func (*LifecycleCallback) PostShell(node *Node, stdin io.WriteCloser) error {
	for i := range node.CallbackShells {
		shell := node.CallbackShells[i]
		time.Sleep(shell.Delay * time.Millisecond)

		if shell.CpShell.Src != "" {
			err := copyFile(shell.CpShell.Src, shell.CpShell.Tgt, stdin)
			if err != nil {
				return err
			}
			continue
		}

		_, _ = stdin.Write([]byte(shell.Cmd + "\r"))
	}
	return nil
}

func (*LifecycleCallback) Priority() int {
	return 0
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
