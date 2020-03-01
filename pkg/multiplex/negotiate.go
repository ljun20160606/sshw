package multiplex

import (
	"os"
	"path"
)

var (
	homeDir, _ = os.UserHomeDir()
	SocketDir  = path.Dir(SocketPath)
	SocketPath = homeDir + "/.config/sshw/sshw.socket"
)
