package sshw

import (
	"bufio"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"os"
	"syscall"
)

func init() {
	RegisterLifecycle(new(LifecyclePassword))
}

var _ Lifecycle = new(LifecyclePassword)

type LifecyclePassword struct {
}

func (*LifecyclePassword) PostShell(node *Node, stdin io.WriteCloser) error {
	return nil
}

func (*LifecyclePassword) PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error {
	password := node.password()

	if password != nil {
		clientConfig.Auth = append(clientConfig.Auth, password)
	}

	clientConfig.Auth = append(clientConfig.Auth, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, 0, len(questions))
		for i, q := range questions {
			fmt.Print(q)
			if echos[i] {
				scan := bufio.NewScanner(os.Stdin)
				if scan.Scan() {
					answers = append(answers, scan.Text())
				}
				err := scan.Err()
				if err != nil {
					return nil, err
				}
			} else {
				b, err := terminal.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return nil, err
				}
				fmt.Println()
				answers = append(answers, string(b))
			}
		}
		return answers, nil
	}))
	return nil
}

func (*LifecyclePassword) PostSSHDial(node *Node, client *ssh.Client) error {
	return nil
}

func (*LifecyclePassword) PostNewSession(node *Node, session *ssh.Session) error {
	return nil
}

func (*LifecyclePassword) Priority() int {
	return 0
}
