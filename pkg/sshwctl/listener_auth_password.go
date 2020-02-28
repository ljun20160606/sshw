package sshwctl

import (
	"bufio"
	"fmt"
	"github.com/dgryski/dgoogauth"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	RegisterLifecycle(&CommonLifecycle{
		Name:                     "auth_password",
		PostInitClientConfigFunc: new(LifecyclePassword).PostInitClientConfig,
	})
}

type LifecyclePassword struct {
}

// if set password, auto auth password
// if set KeyboardInteractions, match question and then auto auth interaction
func (*LifecyclePassword) PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error {
	password := node.password()

	if password != nil {
		clientConfig.Auth = append(clientConfig.Auth, password)
	}

	clientConfig.Auth = append(clientConfig.Auth, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, 0, len(questions))
	QUESTIONS:
		for i, q := range questions {
			fmt.Print(q)
			for i := range node.KeyboardInteractions {
				keyboardInteractive := node.KeyboardInteractions[i]
				if strings.Contains(q, keyboardInteractive.Question) {
					answer := keyboardInteractive.Answer
					if keyboardInteractive.GoogleAuth {
						code := dgoogauth.ComputeCode(keyboardInteractive.Answer, time.Now().Unix()/30)
						fmt.Print(code)
						answer = strconv.Itoa(code)
					}
					answers = append(answers, answer)
					continue QUESTIONS
				}
			}
			if echos[i] {
				scan := bufio.NewScanner(os.Stdin)
				if scan.Scan() {
					answers = append(answers, scan.Text())
				}
				if err := scan.Err(); err != nil {
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
