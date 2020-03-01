package main

import (
	"fmt"
	"github.com/ljun20160606/sshw/pkg/multiplex"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"os/exec"
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
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
)

func init() {
	rootCmd.Flags().BoolP("ssh", "s", false, "use local ssh config '~/.ssh/config'")
	rootCmd.Flags().BoolP("version", "v", false, "show version")
	rootCmd.PersistentFlags().StringP("filename", "f", "", ".sshw config filename")

	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if v := rootCmd.Flags().Lookup("version").Value.String(); v == "true" {
			showVersion()
			return
		}
		var nodes []*sshwctl.Node
		var err error
		if useSsh := rootCmd.Flags().Lookup("ssh").Value.String(); useSsh == "true" {
			if nodes, err = sshwctl.LoadSshConfig(); err != nil {
				fmt.Println("load ssh config", err)
				return
			}
		} else {
			filename := rootCmd.PersistentFlags().Lookup("filename").Value.String()
			if _, nodes, err = sshwctl.LoadYamlConfig(filename); err != nil {
				fmt.Println("load yaml config", err)
				return
			}
		}
		if err := sshwctl.PrepareConfig(nodes); err != nil {
			fmt.Println("prepare config", err)
			return
		}

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
}

func ExecNode(node *sshwctl.Node) error {
	if node.ControlMaster != nil && !*node.ControlMaster {
		return sshwctl.ExecNode(node)
	}
	if !multiplex.IsRunning() {
		path, err := exec.LookPath(os.Args[0])
		if err != nil {
			return err
		}
		file, err := os.OpenFile(multiplex.SocketDir+"/sshw.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
		if err != nil {
			return err
		}
		cmd := exec.Command(path, "server")
		cmd.Stdout = file
		cmd.Stderr = file
		if err := cmd.Start(); err != nil {
			return err
		}
		_ = ioutil.WriteFile(multiplex.SocketDir+"/sshw.pid", []byte(strconv.Itoa(cmd.Process.Pid)), 0755)
	}
	timeout := time.Now().Add(time.Second)
	for {
		if multiplex.IsRunning() {
			return multiplex.ExecNode(node)
		}
		if time.Now().Before(timeout) {
			time.Sleep(30 * time.Millisecond)
			continue
		}
		fmt.Println("can not run daemon server, exec directly")
		return sshwctl.ExecNode(node)
	}
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
