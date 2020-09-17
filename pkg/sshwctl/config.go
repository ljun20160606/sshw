package sshwctl

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
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

var (
	homeDir, _           = os.UserHomeDir()
	SshPath              = path.Join(homeDir, ".ssh/config")
	SshwDir              = path.Join(homeDir, ".config/sshw")
	SshwGlobalConfigPath = path.Join(SshwDir, "config.yaml")
)

type Node struct {
	// Node Name or .ssh/config Host
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
	Src       string `yaml:"src"`
	Tgt       string `yaml:"tgt"`
	IsReceive bool   `yaml:"is-receive"`
	// seconds
	Timeout int64
}

func (n *Node) String() string {
	return n.user() + "@" + n.addr()
}

func (n *Node) user() string {
	envUser := os.Getenv("USER")
	if n.User == "$USER" {
		return envUser
	}
	if n.User == "" {
		return envUser
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

// match .ssh/config Pattern
// if Node.Host == config.Host
// set config.HostName to Node.Host
// same set config.User and config.Port
func MergeSshConfig(nodes []*Node, sshNodes []*Node) error {
	if nodes == nil || sshNodes == nil {
		return nil
	}
	for i := range nodes {
		node := nodes[i]
		if err := MergeSshConfig(node.Children, sshNodes); err != nil {
			return errors.WithMessage(err, "merge ssh")
		}
		if node.Host == "" {
			continue
		}
		for si := range sshNodes {
			sNode := sshNodes[si]
			// sNode.Name is Host Pattern
			if node.Host == sNode.Name {
				node.Host = sNode.Host
				if node.User == "" {
					node.User = sNode.User
				}
				if node.Port == 0 {
					node.Port = sNode.Port
				}
			}
		}
	}
	return nil
}

// render template into nodes
// 1. Parse template ${Env_Variable}
// 2. solve path.convert '*' to absPath
func InitConfig(config interface{}) error {
	if err := WalkInterface(reflect.ValueOf(config), false, func(k string, t reflect.Type, v reflect.Value, structField *reflect.StructField) (stop bool) {
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
	}); err != nil {
		return errors.WithMessage(err, "prepare config")
	}
	return nil
}

// global config
var globalConfig []*Node

func init() {
	_, globalConfig, _ = LoadYamlConfig(SshwGlobalConfigPath)
}

type MatchFunc func(node *Node, globalNode *Node) bool

func MatchCommonConfig(node *Node, globalNode *Node) bool {
	return node.Host == globalNode.Host
}

func MatchSshConfig(node *Node, globalNode *Node) bool {
	return node.Host == globalNode.Host || node.Name == globalNode.Host
}

// update nodes based on global config
// if host is equal, update node
// only iterate first level
func InitNodesBaseOnGlobal(nodes []*Node, matchFunc MatchFunc) {
	if nodes == nil || globalConfig == nil {
		return
	}
	for i := range nodes {
		node := nodes[i]
		InitNodesBaseOnGlobal(node.Children, matchFunc)
		for si := range globalConfig {
			globalNode := globalConfig[si]
			if matchFunc(node, globalNode) {
				fillIfEmpty(node, globalNode)
			}
		}
	}
}

// fill the properties of configuration type
func fillIfEmpty(node, sNode *Node) {
	if node.User == "" {
		node.User = sNode.User
	}
	if node.Port == 0 {
		node.Port = sNode.Port
	}
	if node.Password == "" {
		node.Password = sNode.Password
	}
	if len(node.KeyboardInteractions) == 0 {
		node.KeyboardInteractions = sNode.KeyboardInteractions
	}
	if node.ControlMaster == nil {
		node.ControlMaster = sNode.ControlMaster
	}
	if node.KeyPath == "" {
		node.KeyPath = sNode.KeyPath
	}
	if node.Passphrase == "" {
		node.Passphrase = sNode.Passphrase
	}
}

// return filepath and nodes, load config in filename
func LoadYamlConfig(filename string) (string, []*Node, error) {
	pathname, b, err := ReadConfigBytes(filename)
	if err != nil {
		return "", nil, errors.WithMessage(err, "load yaml")
	}

	if nodes, err := LoadYamlConfig0(b); err != nil {
		return "", nil, errors.WithMessage(err, "load yaml")
	} else {
		return pathname, nodes, nil
	}
}

// return filepath and bytes of config
func ReadConfigBytes(filename string) (string, []byte, error) {
	// default
	if filename == "" {
		pathname, b, err := ReadDefaultConfigBytes(".sshw", ".sshw.yml", ".sshw.yaml")
		if err != nil {
			return "", nil, err
		}
		return pathname, b, nil
	}
	// as url, if filename is /absolute/filename, err is nil, so need check Host
	if uri, err := url.ParseRequestURI(filename); err == nil && uri.Host != "" {
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

// AbsPath returns absolute path and match wild path
// if match multiple path, return first
func AbsPath(input string) string {
	p := input
	if p == "" {
		return ""
	}
	if p[0] == '~' {
		p = path.Join(homeDir, p[1:])
	}
	matches, _ := filepath.Glob(p)
	if len(matches) != 0 {
		p = matches[0]
	}
	abs, _ := filepath.Abs(p)
	return abs
}

// return yaml config
func LoadYamlConfig0(bs []byte) ([]*Node, error) {
	var result []*Node
	configLoader := NewYamlConfigLoader(bs)
	if err := configLoader.Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// return config of .ssh/config
func LoadSshConfig() ([]*Node, error) {
	f, err := os.Open(SshPath)
	var nc []*Node
	if err != nil {
		if os.IsNotExist(err) {
			return nc, nil
		}
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	loader := NewSshConfigLoader(f)
	if err := loader.Decode(&nc); err != nil {
		return nil, err
	}
	return nc, nil
}

type ConfigLoader interface {
	Decode(nodes *[]*Node) error
}

type YamlConfigLoader struct {
	bs []byte
}

func NewYamlConfigLoader(bs []byte) ConfigLoader {
	return &YamlConfigLoader{bs: bs}
}

func (y *YamlConfigLoader) Decode(nodes *[]*Node) error {
	reader1 := bytes.NewReader(y.bs)
	reader2 := bytes.NewReader(y.bs)
	{
		var e error
		decoder1 := yaml.NewDecoder(reader1)
		decoder2 := yaml.NewDecoder(reader2)
		for {
			n := new(Node)
			if err := decoder1.Decode(n); err != nil {
				if err == io.EOF {
					return nil
				}
				e = err
			} else {
				if n.Name == "" {
					continue
				}
				*nodes = append(*nodes, n)
			}

			var ns []*Node
			if err := decoder2.Decode(&ns); err != nil {
				if err == io.EOF {
					return nil
				}
				if e != nil {
					return e
				}
			} else {
				*nodes = append(*nodes, ns...)
			}
		}
	}
}

type SshConfigLoader struct {
	r io.Reader
}

func NewSshConfigLoader(r io.Reader) ConfigLoader {
	return &SshConfigLoader{r: r}
}

func (s *SshConfigLoader) Decode(nodes *[]*Node) error {
	cfg, err := ssh_config.Decode(s.r)
	if err != nil {
		return err
	}
	for _, host := range cfg.Hosts {
		alias := fmt.Sprintf("%s", host.Patterns[0])
		hostName, err := cfg.Get(alias, "HostName")
		if err != nil {
			return errors.WithMessage(err, "load ssh")
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
			*nodes = append(*nodes, c)
		}
	}
	return nil
}

func ReadDefaultConfigBytes(names ...string) (string, []byte, error) {
	// homedir
	for i := range names {
		pathname := path.Join(homeDir, names[i])
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
	return "", nil, nil
}

// 1. load global yaml
// 2. render template
// 3. load .ssh/config
func InitNodes(nodes []*Node) error {
	// 1
	InitNodesBaseOnGlobal(nodes, MatchCommonConfig)
	// 2
	if err := InitConfig(nodes); err != nil {
		return err
	}
	// 3
	sshNodes, _ := LoadSshConfig()
	if err := MergeSshConfig(nodes, sshNodes); err != nil {
		return err
	}
	return nil
}

// 1. load global yaml
// 2. render template
func InitSshNodes(nodes []*Node) error {
	// 1
	InitNodesBaseOnGlobal(nodes, MatchSshConfig)
	// 2
	if err := InitConfig(nodes); err != nil {
		return err
	}
	return nil
}
