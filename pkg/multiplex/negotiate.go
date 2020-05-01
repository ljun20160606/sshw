package multiplex

import (
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"path"
)

var (
	SocketPath = path.Join(sshwctl.SshwDir, "sshw.socket")
)
