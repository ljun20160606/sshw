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
	Alias          string               `yaml:"alias"`
	ExecsPre       []*NodeExec          `yaml:"execs-pre"`
	ExecsStop      []*NodeExec          `yaml:"execs-stop"`
	Host           string               `yaml:"host"`
	User           string               `yaml:"user"`
	Port           int                  `yaml:"port"`
	KeyPath        string               `yaml:"keypath"`
	Passphrase     string               `yaml:"passphrase"`
	Password       string               `yaml:"password"`
	CallbackShells []*NodeCallbackShell `yaml:"callback-shells"`
	Children       []*Node              `yaml:"children"`
	Jump           []*Node              `yaml:"jump"`
}

type NodeExec struct {
	Cmd string `yaml:"cmd"`
}

type NodeCallbackShell struct {
	Cmd          string        `yaml:"cmd"`
	CpShell      NodeCp        `yaml:"cp"`
	Delay        time.Duration `yaml:"delay"`
	ErrorPattern string        `yaml:"error-pattern"`
	Wait         time.Duration `yaml:"wait"`
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

var (
	config []*Node
)

func GetConfig() []*Node {
	return config
}

func PrepareConfig() error {
	return WalkInterface(reflect.ValueOf(config), false, func(k string, t reflect.Type, v reflect.Value) {
		if t.Kind() != reflect.String || !v.CanSet() {
			return
		}
		r := ParseSshwTemplate(v.Interface().(string)).Execute()
		v.Set(reflect.ValueOf(r))
	})
}

func LoadYamlConfig(filename string) error {
	var b []byte
	var err error
	if filename != "" {
		b, err = ioutil.ReadFile(naiveRealpath(filename))
	} else {
		b, err = LoadConfigBytes(".sshw", ".sshw.yml", ".sshw.yaml")
	}

	if err != nil {
		return err
	}
	var c []*Node
	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return err
	}

	config = c

	return nil
}

func LoadSshConfig() error {
	u, err := user.Current()
	if err != nil {
		l.Error(err)
		return nil
	}
	f, _ := os.Open(path.Join(u.HomeDir, ".ssh/config"))
	cfg, _ := ssh_config.Decode(f)
	var nc []*Node
	for _, host := range cfg.Hosts {
		alias := fmt.Sprintf("%s", host.Patterns[0])
		hostName, err := cfg.Get(alias, "HostName")
		if err != nil {
			return err
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
	config = nc
	return nil
}

func LoadConfigBytes(names ...string) ([]byte, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	// homedir
	for i := range names {
		sshw, err := ioutil.ReadFile(path.Join(u.HomeDir, names[i]))
		if err == nil {
			return sshw, nil
		}
	}
	// relative
	for i := range names {
		sshw, err := ioutil.ReadFile(names[i])
		if err == nil {
			return sshw, nil
		}
	}
	return nil, err
}
