package sshwctl

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"github.com/atrox/homedir"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

type Node struct {
	Name                 string                `yaml:"name"`
	Alias                string                `yaml:"alias,omitempty"`
	ExecsPre             []*NodeExec           `yaml:"execs-pre,omitempty"`
	ExecsStop            []*NodeExec           `yaml:"execs-stop,omitempty"`
	Host                 string                `yaml:"host,omitempty"`
	User                 string                `yaml:"user,omitempty"`
	Port                 int                   `yaml:"port,omitempty"`
	KeyPath              string                `yaml:"keypath,omitempty"`
	Passphrase           string                `yaml:"passphrase,omitempty"`
	Password             string                `yaml:"password,omitempty"`
	CallbackShells       []*NodeCallbackShell  `yaml:"callback-shells,omitempty"`
	Scps                 []*NodeCp             `yaml:"scps"`
	Children             []*Node               `yaml:"children,omitempty"`
	Jump                 []*Node               `yaml:"jump,omitempty"`
	MergeIgnore          bool                  `yaml:"merge-ignore,omitempty"`
	KeyboardInteractions []KeyboardInteractive `yaml:"keyboard-interactions"`
	ControlMaster        *bool                 `yaml:"control-master"`

	Stdin   io.ReadCloser   `yaml:"-"`
	Stdout  io.Writer       `yaml:"-"`
	Stderr  io.Writer       `yaml:"-"`
	Width   int             `yaml:"-"`
	Height  int             `yaml:"-"`
	State   *terminal.State `yaml:"-"`
	Session *ssh.Session    `yaml:"-"`
}

func (n *Node) stdin() io.ReadCloser {
	if n.Stdin != nil {
		return n.Stdin
	}
	return os.Stdin
}

func (n *Node) stdout() io.Writer {
	if n.Stdout != nil {
		return n.Stdout
	}
	return os.Stdout
}

func (n *Node) stderr() io.Writer {
	if n.Stderr != nil {
		return n.Stderr
	}
	return os.Stderr
}

func (n *Node) Print(message string) {
	_, _ = n.stdout().Write([]byte(message))
}

func (n *Node) Println(message string) {
	_, _ = n.stdout().Write([]byte(message + "\n"))
}

func (n *Node) Error(err error) {
	_, _ = n.stderr().Write([]byte(err.Error()))
}

// when it have KeyboardInteractive
// sshw will answer question if question contains content that we set.
// if AnswerAll is true, it will don't match question
type KeyboardInteractive struct {
	Question   string
	Answer     string
	GoogleAuth bool `yaml:"google-auth"`
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
		dstNode := dst[nodeIndex]
		if IsBookmark(dstNode) && IsBookmark(srcNode) {
			MergeNodes(&dstNode.Children, srcNode.Children)
		} else {
			dst[nodeIndex] = srcNode
		}
	}
	*dstPtr = append(dst, canMerged...)
}

// only name and children have value
// etc.
// - name: foo
//   children:
//    - name: bar
//    - name: zoo
func IsBookmark(n *Node) bool {
	notEmptyNames, _ := FieldsNotEmpty(n, []string{"Name", "Children", "MergeIgnore"})
	return len(notEmptyNames) == 0
}

type NodeExec struct {
	Cmd string `yaml:"cmd"`
	Var string `yaml:"var"`
}

type NodeCallbackShell struct {
	Cmd          string        `yaml:"cmd"`
	Delay        time.Duration `yaml:"delay,omitempty"`
	ErrorPattern string        `yaml:"error-pattern,omitempty"`
	Wait         time.Duration `yaml:"wait,omitempty"`
}

type NodeCp struct {
	Src string `yaml:"src" sshw:"path"`
	Tgt string `yaml:"tgt"`
	// seconds
	Timeout int64
}

func (n *Node) String() string {
	return n.user() + "@" + n.addr()
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

// render template into nodes
func PrepareConfig(config interface{}) error {
	return WalkInterface(reflect.ValueOf(config), false, func(k string, t reflect.Type, v reflect.Value, structField *reflect.StructField) (stop bool) {
		if t.Kind() != reflect.String || !v.CanSet() {
			return
		}

		r := ParseSshwTemplate(v.Interface().(string)).Execute()

		if structField != nil {
			switch tagSshw := structField.Tag.Get("sshw"); tagSshw {
			case "path":
				r = AbsPath(r)
			}
		}
		v.Set(reflect.ValueOf(r))
		return
	})
}

func LoadYamlConfig(filename string) (string, []*Node, error) {
	pathname, b, err := ReadConfigBytes(filename)
	if err != nil {
		return "", nil, err
	}

	if nodes, err := LoadConfig(b); err != nil {
		return "", nil, err
	} else {
		return pathname, nodes, nil
	}
}

func ReadConfigBytes(filename string) (string, []byte, error) {
	// default
	if filename == "" {
		pathname, b, err := ReadDefaultConfigBytes(".sshw", ".sshw.yml", ".sshw.yaml")
		if err != nil {
			return "", nil, err
		}
		return pathname, b, nil
	}
	// as url
	if _, err := url.ParseRequestURI(filename); err == nil {
		if response, err := http.Get(filename); err != nil {
			return "", nil, err
		} else {
			defer func() {
				_ = response.Body.Close()
			}()
			if b, err := ioutil.ReadAll(response.Body); err != nil {
				return "", nil, err
			} else {
				return filename, b, nil
			}
		}
	}
	// specify path
	pathname := AbsPath(filename)
	b, err := ioutil.ReadFile(pathname)
	if err != nil {
		return "", nil, err
	}
	return pathname, b, nil
}

func LoadConfig(bs []byte) ([]*Node, error) {
	var result []*Node

	reader1 := bytes.NewReader(bs)
	reader2 := bytes.NewReader(bs)
	{
		var e error
		decoder1 := yaml.NewDecoder(reader1)
		decoder2 := yaml.NewDecoder(reader2)
		for {
			n := new(Node)
			if err := decoder1.Decode(n); err != nil {
				if err == io.EOF {
					return result, nil
				}
				e = err
			} else {
				if n.Name == "" {
					continue
				}
				result = append(result, n)
			}

			var nodes []*Node
			if err := decoder2.Decode(&nodes); err != nil {
				if err == io.EOF {
					return result, nil
				}
				if e != nil {
					return result, e
				}
			} else {
				result = append(result, nodes...)
			}
		}
	}
}

// AbsPath returns absolute path and match wild path
// if match multiple path, return first
func AbsPath(input string) string {
	p := input
	if p == "" {
		return ""
	}
	if p[0] == '~' {
		u, _ := user.Current()
		p = path.Join(u.HomeDir, p[1:])
	}
	matches, _ := filepath.Glob(p)
	if len(matches) != 0 {
		p = matches[0]
	}
	abs, _ := filepath.Abs(p)
	return abs
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

func ReadDefaultConfigBytes(names ...string) (string, []byte, error) {
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
