package multiplex

import (
	"os"
	"path"
)

var (
	homeDir, _ = os.UserHomeDir()
	SocketDir  = path.Dir(SocketPath)
	SocketPath = path.Join(homeDir, ".config/sshw/sshw.socket")
)
