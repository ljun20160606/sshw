package sshw

import (
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"os/user"
	"path"
)

func init() {
	RegisterLifecycle(new(LifecyclePassword))
}

var _ Lifecycle = new(LifecyclePem)

type LifecyclePem struct {
}

func (*LifecyclePem) PostShell(node *Node, stdin io.WriteCloser) error {
	return nil
}

func (*LifecyclePem) PostInitClientConfig(node *Node, clientConfig *ssh.ClientConfig) error {
	u, err := user.Current()
	if err != nil {
		l.Error(err)
		return nil
	}
	var pemBytes []byte
	if node.KeyPath == "" {
		pemBytes, err = ioutil.ReadFile(path.Join(u.HomeDir, ".ssh/id_rsa"))
	} else {
		pemBytes, err = ioutil.ReadFile(node.KeyPath)
	}
	if err != nil {
		l.Error(err)
	} else {
		var signer ssh.Signer
		if node.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(node.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(pemBytes)
		}
		if err != nil {
			l.Error(err)
		} else {
			clientConfig.Auth = append(clientConfig.Auth, ssh.PublicKeys(signer))
		}
	}
	return nil
}

func (*LifecyclePem) PostSSHDial(node *Node, client *ssh.Client) error {
	return nil
}

func (*LifecyclePem) PostNewSession(node *Node, session *ssh.Session) error {
	return nil
}

func (*LifecyclePem) Priority() int {
	return 0
}
