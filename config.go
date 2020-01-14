package sshw

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path"
	"reflect"
	"strconv"
	"time"

	"github.com/atrox/homedir"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

type Node struct {
	Name           string               `yaml:"name"`
	Alias          string               `yaml:"alias,omitempty"`
	ExecsPre       []*NodeExec          `yaml:"execs-pre,omitempty"`
	ExecsStop      []*NodeExec          `yaml:"execs-stop,omitempty"`
	Host           string               `yaml:"host,omitempty"`
	User           string               `yaml:"user,omitempty"`
	Port           int                  `yaml:"port,omitempty"`
	KeyPath        string               `yaml:"keypath,omitempty"`
	Passphrase     string               `yaml:"passphrase,omitempty"`
	Password       string               `yaml:"password,omitempty"`
	CallbackShells []*NodeCallbackShell `yaml:"callback-shells,omitempty"`
	Children       []*Node              `yaml:"children,omitempty"`
	Jump           []*Node              `yaml:"jump,omitempty"`
	MergeIgnore    bool                 `yaml:"merge-ignore,omitempty"`
}

// merge srcNode to dstNode
// only compare name and override, otherwise it is complex.
func MergeNodes(dstPtr *[]*Node, src []*Node) {
	dst := *dstPtr
	var canMerged []*Node
	for srcIndex := range src {
		srcNode := src[srcIndex]
		if srcNode.MergeIgnore {
			continue
		}
		nodeIndex := -1
		for dstIndex := range dst {
			dstNode := dst[dstIndex]
			if srcNode.Name == dstNode.Name {
				nodeIndex = dstIndex
				break
			}
		}
		if nodeIndex < 0 {
			canMerged = append(canMerged, srcNode)
			continue
		}
		dst[nodeIndex] = srcNode
	}
	*dstPtr = append(dst, canMerged...)
}

type NodeExec struct {
	Cmd string `yaml:"cmd"`
}

type NodeCallbackShell struct {
	Cmd          string        `yaml:"cmd"`
	CpShell      NodeCp        `yaml:"cp,omitempty"`
	Delay        time.Duration `yaml:"delay,omitempty"`
	ErrorPattern string        `yaml:"error-pattern,omitempty"`
	Wait         time.Duration `yaml:"wait,omitempty"`
}

type NodeCp struct {
	Src string `yaml:"src"`
	Tgt string `yaml:"tgt"`
}

func (n *Node) String() string {
	return n.Name
}

func (n *Node) user() string {
	if n.User == "" {
		return "root"
	}
	if n.User == "$USER" {
		return os.Getenv("USER")
	}
	return n.User
}

func (n *Node) port() int {
	if n.Port <= 0 {
		return 22
	}
	return n.Port
}

func (n *Node) portStr() string {
	return strconv.Itoa(n.port())
}

func (n *Node) addr() string {
	return net.JoinHostPort(n.Host, n.portStr())
}

func (n *Node) password() ssh.AuthMethod {
	if n.Password == "" {
		return nil
	}
	return ssh.Password(n.Password)
}

func (n *Node) alias() string {
	return n.Alias
}

func PrepareConfig(config []*Node) error {
	return WalkInterface(reflect.ValueOf(config), false, func(k string, t reflect.Type, v reflect.Value) {
		if t.Kind() != reflect.String || !v.CanSet() {
			return
		}
		r := ParseSshwTemplate(v.Interface().(string)).Execute()
		v.Set(reflect.ValueOf(r))
	})
}

func LoadYamlConfig(filename string) (string, []*Node, error) {
	var b []byte
	var err error
	var pathname string
	if filename != "" {
		pathname = naiveRealpath(filename)
		b, err = ioutil.ReadFile(pathname)
	} else {
		pathname, b, err = LoadConfigBytes(".sshw", ".sshw.yml", ".sshw.yaml")
	}

	if err != nil {
		return pathname, nil, err
	}
	var c []*Node
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return pathname, nil, err
	}

	return pathname, c, nil
}

func LoadSshConfig() ([]*Node, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	f, _ := os.Open(path.Join(u.HomeDir, ".ssh/config"))
	cfg, _ := ssh_config.Decode(f)
	var nc []*Node
	for _, host := range cfg.Hosts {
		alias := fmt.Sprintf("%s", host.Patterns[0])
		hostName, err := cfg.Get(alias, "HostName")
		if err != nil {
			return nil, err
		}
		if hostName != "" {
			port, _ := cfg.Get(alias, "Port")
			var c = new(Node)
			c.Name = alias
			c.Alias = alias
			c.Host = hostName
			c.User, _ = cfg.Get(alias, "User")
			c.Port, _ = strconv.Atoi(port)
			keyPath, _ := cfg.Get(alias, "IdentityFile")
			c.KeyPath, _ = homedir.Expand(keyPath)
			nc = append(nc, c)
		}
	}
	return nc, nil
}

func LoadConfigBytes(names ...string) (string, []byte, error) {
	u, err := user.Current()
	if err != nil {
		return "", nil, err
	}
	// homedir
	for i := range names {
		pathname := path.Join(u.HomeDir, names[i])
		configBytes, err := ioutil.ReadFile(pathname)
		if err == nil {
			return pathname, configBytes, nil
		}
	}
	// relative
	for i := range names {
		pathname := names[i]
		configBytes, err := ioutil.ReadFile(pathname)
		if err == nil {
			return pathname, configBytes, nil
		}
	}
	return "", nil, err
}
