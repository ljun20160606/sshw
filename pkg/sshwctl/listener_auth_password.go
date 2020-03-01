package sshwctl

import (
	"bufio"
	"github.com/dgryski/dgoogauth"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"strconv"
	"strings"
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
			node.Print(q)
			for interactIndex := range node.KeyboardInteractions {
				keyboardInteractive := node.KeyboardInteractions[interactIndex]
				if strings.Contains(q, keyboardInteractive.Question) {
					answer := keyboardInteractive.Answer
					if keyboardInteractive.GoogleAuth {
						code := dgoogauth.ComputeCode(keyboardInteractive.Answer, time.Now().Unix()/30)
						answer = strconv.Itoa(code)
						node.Print(answer)
					}
					answers = append(answers, answer)
					continue QUESTIONS
				}
			}
			if echos[i] {
				scan := bufio.NewScanner(node.stdin())
				if scan.Scan() {
					answers = append(answers, scan.Text())
				}
				if err := scan.Err(); err != nil {
					return nil, err
				}
			} else {
				// todo agent
				b, err := terminal.ReadPassword(int(os.Stdin.Fd()))
				if err != nil {
					return nil, err
				}
				node.Println("")
				answers = append(answers, string(b))
			}
		}
		return answers, nil
	}))
	return nil
}
