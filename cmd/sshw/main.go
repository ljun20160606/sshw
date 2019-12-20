package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/ljun20160606/sshw"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const prev = "-parent-"

var (
	log       = sshw.GetLogger()
	templates = &promptui.SelectTemplates{
		Label:    "✨ {{ . | green}}",
		Active:   "➤ {{ .Name | cyan  }}{{if .Alias}}({{.Alias | yellow}}){{end}} {{if .Host}}{{if .User}}{{.User | faint}}{{`@` | faint}}{{end}}{{.Host | faint}}{{end}}",
		Inactive: "  {{.Name | faint}}{{if .Alias}}({{.Alias | faint}}){{end}} {{if .Host}}{{if .User}}{{.User | faint}}{{`@` | faint}}{{end}}{{.Host | faint}}{{end}}",
	}
	Version string
)

var (
	githubRepository = &sshw.GithubRepository{
		Url:      "https://github.com",
		Username: "ljun20160606",
		Name:     "sshw",
	}
	rootCmd   = &cobra.Command{}
	latestCmd = &cobra.Command{
		Use:   "latest",
		Short: "get latest version in remote",
		Run: func(cmd *cobra.Command, args []string) {
			version, err := githubRepository.LatestVersion()
			if err != nil {
				log.Error(err)
				return
			}
			fmt.Println("latest version:", version)
		},
	}
)

func init() {
	rootCmd.PersistentFlags().BoolP("ssh", "s", false, "use local ssh config '~/.ssh/config'")
	rootCmd.PersistentFlags().StringP("filename", "f", "", ".sshw config filename")
	rootCmd.PersistentFlags().BoolP("version", "v", false, "show version")

	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if v := rootCmd.PersistentFlags().Lookup("version").Value.String(); v == "true" {
			fmt.Println("sshw - ssh client wrapper for automatic login")
			fmt.Println("go version:", runtime.Version())
			fmt.Println("version:", Version)
			return
		}
		if useSsh := rootCmd.PersistentFlags().Lookup("ssh").Value.String(); useSsh == "true" {
			if err := sshw.LoadSshConfig(); err != nil {
				log.Error("load ssh config", err)
				return
			}
		} else {
			filename := rootCmd.PersistentFlags().Lookup("filename").Value.String()
			if err := sshw.LoadYamlConfig(filename); err != nil {
				log.Error("load yaml config", err)
				return
			}
		}
		if err := sshw.PrepareConfig(); err != nil {
			log.Error("prepare config", err)
			return
		}

		var nodes = sshw.GetConfig()

		// login by alias
		if len(args) >= 1 {
			var nodeAlias = args[0]
			var node = findAlias(nodes, nodeAlias)
			if node != nil {
				client := sshw.NewClient(node)
				if err := client.Login(); err != nil {
					log.Error(err)
				}
				return
			}
		}

		node := choose(nil, sshw.GetConfig())
		if node == nil {
			return
		}

		client := sshw.NewClient(node)
		if err := client.Login(); err != nil {
			log.Error(err)
		}
	}
	rootCmd.AddCommand(latestCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func findAlias(nodes []*sshw.Node, nodeAlias string) *sshw.Node {
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

func choose(parent, trees []*sshw.Node) *sshw.Node {
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
				node := (*scopeAll[i]).(sshw.Node)
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

	var node *sshw.Node
	if searched {
		n := (*scope[index]).(sshw.Node)
		node = &n
	} else {
		node = trees[index]
	}

	if len(node.Children) > 0 {
		first := node.Children[0]
		if first.Name != prev {
			first = &sshw.Node{Name: prev}
			node.Children = append(node.Children[:0], append([]*sshw.Node{first}, node.Children...)...)
		}
		return choose(trees, node.Children)
	}

	if node.Name == prev {
		if parent == nil {
			return choose(nil, sshw.GetConfig())
		}
		return choose(nil, parent)
	}

	return node
}

func deepSearch(trees []*sshw.Node) []*interface{} {
	var scope []*interface{}
	for i := range trees {
		deepSearchHelper(trees[i], &scope)
	}
	return scope
}

func deepSearchHelper(node *sshw.Node, scope *[]*interface{}) {
	if node == nil {
		return
	}
	var tmp interface{} = *node
	*scope = append(*scope, &tmp)
	for i := range node.Children {
		deepSearchHelper(node.Children[i], scope)
	}
}

func searchMatch(input string, node *sshw.Node) bool {
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
