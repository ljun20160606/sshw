package main

import (
	"context"
	"fmt"
	"github.com/ljun20160606/sshw/pkg/multiplex"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

const prev = "-parent-"

var (
	templates = &promptui.SelectTemplates{
		Label:    "✨ {{ . | green}}",
		Active:   "➤ {{ .Name | cyan  }}{{if .Alias}}({{.Alias | yellow}}){{end}} {{if .Host}}{{if .User}}{{.User | faint}}{{`@` | faint}}{{end}}{{.Host | faint}}{{end}}",
		Inactive: "  {{.Name | faint}}{{if .Alias}}({{.Alias | faint}}){{end}} {{if .Host}}{{if .User}}{{.User | faint}}{{`@` | faint}}{{end}}{{.Host | faint}}{{end}}",
	}
	Version string
)

var (
	rootCmd = &cobra.Command{
		Use: "sshw",
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
)

func init() {
	rootCmd.Flags().BoolP("ssh", "s", false, "use local ssh config '~/.ssh/config'")
	rootCmd.Flags().BoolP("version", "v", false, "show version")
	rootCmd.PersistentFlags().StringP("filename", "f", "", ".sshw config. filename or url")

	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if v := rootCmd.Flags().Lookup("version").Value.String(); v == "true" {
			showVersion()
			return
		}
		nodes, err := NewNodes(NewNodesLoaderConfig())
		if err != nil {
			fmt.Println(err)
			return
		}
		FindAndRun(nodes, args)
	}
}

type NodesLoaderConfig struct {
	// if true only use ssh config as nodes
	useSsh bool
	// .sshw.yaml path
	filename string
}

func NewNodesLoaderConfig() *NodesLoaderConfig {
	return &NodesLoaderConfig{
		useSsh:   rootCmd.Flags().Lookup("ssh").Value.String() == "true",
		filename: rootCmd.PersistentFlags().Lookup("filename").Value.String(),
	}
}

// 1. load -f yaml
// 2. load global yaml
// 3. render template
// 4. load .ssh/config. Do it last because there is not env variable in ssh config
func NewNodes(conf *NodesLoaderConfig) ([]*sshwctl.Node, error) {
	if conf.useSsh {
		sshNodes, sshErr := sshwctl.LoadSshConfig()
		if sshErr != nil {
			return nil, sshErr
		}
		if err := sshwctl.InitSshNodes(sshNodes); err != nil {
			return nil, err
		}
		return sshNodes, nil
	}
	_, nodes, err := sshwctl.LoadYamlConfig(conf.filename)
	if err != nil {
		return nil, err
	}
	if err := sshwctl.InitNodes(nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func FindAndRun(nodes []*sshwctl.Node, args []string) {
	// login by alias
	if len(args) >= 1 {
		var nodeAlias = args[0]
		var node = findAlias(nodes, nodeAlias)
		if node != nil {
			if err := ExecNode(node); err != nil {
				fmt.Println(err)
			}
			return
		}
	}

	node := choose(nodes, nil, nodes)
	if node == nil {
		return
	}

	if err := ExecNode(node); err != nil {
		fmt.Println(err)
	}
}

var (
	SSHWLogPath = path.Join(sshwctl.SshwDir, "sshw.log")
	SSHWPidPath = path.Join(sshwctl.SshwDir, "sshw.pid")
)

func PersistPid(pid int) {
	_ = ioutil.WriteFile(SSHWPidPath, []byte(strconv.Itoa(pid)), 0755)
}

func ReadPid() (int, bool) {
	file, err := ioutil.ReadFile(SSHWPidPath)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(string(file))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func ExecNode(node *sshwctl.Node) error {
	if len(node.Scps) != 0 {
		for i := range node.Scps {
			cp := node.Scps[i]
			if cp.IsReceive {
				cp.Tgt = sshwctl.AbsPath(cp.Tgt)
			} else {
				cp.Src = sshwctl.AbsPath(cp.Src)
			}
		}
	}

	if err := sshwctl.AutoSSHAgent(); err != nil {
		if !sshwctl.UserIdRsaIsNotExist() {
			return err
		}
	}
	if node.ControlMaster != nil && !*node.ControlMaster {
		client := sshwctl.NewClient(node)
		return ExecClient(client, node)
	}
	if !multiplex.IsRunning() {
		if err := multiplex.Setup(); err != nil {
			return err
		}
		file, err := os.OpenFile(SSHWLogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
		if err != nil {
			return err
		}
		lookPath, err := exec.LookPath(os.Args[0])
		if err != nil {
			return err
		}
		cmd := exec.Command(lookPath, "server", "start")
		cmd.Stdout = file
		cmd.Stderr = file
		if err := cmd.Start(); err != nil {
			return err
		}
		PersistPid(cmd.Process.Pid)
	}
	timeout := time.Now().Add(time.Second)
	for {
		if multiplex.IsRunning() {
			client := multiplex.NewClient(node)
			return ExecClient(client, node)
		}
		if time.Now().Before(timeout) {
			time.Sleep(30 * time.Millisecond)
			continue
		}
		fmt.Println("can not run daemon server, exec directly")
		client := sshwctl.NewClient(node)
		return ExecClient(client, node)
	}
}

func ExecClient(client sshwctl.Client, node *sshwctl.Node) error {
	if err := client.ExecsPre(); err != nil {
		return err
	}
	if !client.CanConnect() {
		return nil
	}
	if err := client.Connect(); err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()
	if err := client.InitTerminal(); err != nil {
		return err
	}
	defer client.RecoverTerminal()
	client.WatchWindowChange(func(ch, cw int) error {
		if node.Session != nil {
			return node.Session.WindowChange(ch, cw)
		}
		return nil
	})

	if err := client.Scp(context.Background()); err != nil {
		return err
	}

	if len(node.Scps) != 0 && len(node.CallbackShells) == 0 {
		return nil
	}
	if err := client.Shell(); err != nil {
		return err
	}
	if err := client.ExecsPost(); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func findAlias(nodes []*sshwctl.Node, nodeAlias string) *sshwctl.Node {
	for _, node := range nodes {
		if node.Alias == nodeAlias {
			return node
		}
		if len(node.Children) > 0 {
			alias := findAlias(node.Children, nodeAlias)
			if alias != nil {
				return alias
			}
		}
	}
	return nil
}

func choose(root, parent, trees []*sshwctl.Node) *sshwctl.Node {
	scopeAll := deepSearch(trees)
	var scope []*interface{}
	var searched bool

	prompt := promptui.Select{
		Label:        "select host",
		Items:        trees,
		Templates:    templates,
		Size:         20,
		HideSelected: true,
		CustomSearch: func(input string, items []*interface{}) []*interface{} {
			if input == "" {
				searched = false
				return items
			}
			scope = []*interface{}{}
			for i := range scopeAll {
				node := (*scopeAll[i]).(sshwctl.Node)
				if searchMatch(input, &node) {
					var tmp interface{} = node
					scope = append(scope, &tmp)
				}
			}
			searched = true
			return scope
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		return nil
	}

	var node *sshwctl.Node
	if searched {
		n := (*scope[index]).(sshwctl.Node)
		node = &n
	} else {
		node = trees[index]
	}

	if len(node.Children) > 0 {
		first := node.Children[0]
		if first.Name != prev {
			first = &sshwctl.Node{Name: prev}
			node.Children = append(node.Children[:0], append([]*sshwctl.Node{first}, node.Children...)...)
		}
		return choose(root, trees, node.Children)
	}

	if node.Name == prev {
		if parent == nil {
			return choose(root, nil, root)
		}
		return choose(root, nil, parent)
	}

	return node
}

func deepSearch(trees []*sshwctl.Node) []*interface{} {
	var scope []*interface{}
	for i := range trees {
		deepSearchHelper(trees[i], &scope)
	}
	return scope
}

func deepSearchHelper(node *sshwctl.Node, scope *[]*interface{}) {
	if node == nil {
		return
	}
	var tmp interface{} = *node
	*scope = append(*scope, &tmp)
	for i := range node.Children {
		deepSearchHelper(node.Children[i], scope)
	}
}

func searchMatch(input string, node *sshwctl.Node) bool {
	content := fmt.Sprintf("%s %s %s", node.Name, node.User, node.Host)
	if strings.Contains(input, " ") {
		for _, key := range strings.Split(input, " ") {
			key = strings.TrimSpace(key)
			if key != "" {
				if !strings.Contains(content, key) {
					return false
				}
			}
		}
		return true
	}
	if strings.Contains(content, input) {
		return true
	}
	return false
}
