package sshw

import (
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os/user"
	"path"
)

func init() {
	RegisterLifecycle(&CommonLifecycle{
		Name:                     "auth_pem",
		PostInitClientConfigFunc: new(LifecyclePem).PostInitClientConfig,
	})
}

type LifecyclePem struct {
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
