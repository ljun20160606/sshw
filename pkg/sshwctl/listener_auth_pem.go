package sshwctl

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
)

func init() {
	_ = bus.Subscribe(PostInitClientConfig, AuthPemPostInitClientConfig)
}

func AuthPemPostInitClientConfig(ctx *EventContext, clientConfig *ssh.ClientConfig) {
	node := ctx.Node
	var pemBytes []byte
	var err error
	if node.KeyPath == "" {
		pemBytes, err = ioutil.ReadFile(userIdRsa)
	} else {
		pemBytes, err = ioutil.ReadFile(node.KeyPath)
	}
	if err != nil {
		fmt.Println(err)
	} else {
		var signer ssh.Signer
		if node.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(node.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(pemBytes)
		}
		if err != nil {
			fmt.Println(err)
		} else {
			clientConfig.Auth = append(clientConfig.Auth, ssh.PublicKeys(signer))
		}
	}
}
