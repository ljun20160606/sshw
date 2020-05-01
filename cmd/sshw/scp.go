package main

import (
	"fmt"
	"github.com/alecthomas/participle"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(scpCmd)
}

// 1. host:path
// 2. host:
// 3. user@host:path
// 4. user@host:
type ScpValueGrammar struct {
	User string `parser:"(@Ident \"@\")?"`
	Host string `parser:"@Ident \":\" "`
	Path string `parser:"@Ident?"`
}

func ParseScpValue(input string) (*ScpValueGrammar, error) {
	parser, err := participle.Build(&ScpValueGrammar{})
	if err != nil {
		return nil, err
	}
	ast := &ScpValueGrammar{}
	if err := parser.ParseString(input, ast); err != nil {
		return nil, err
	}
	return ast, nil
}

var scpCmd = &cobra.Command{
	Use:     "scp",
	Short:   "like scp",
	Example: "sshw scp file user@host:",
	Args:    cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		src := args[0]
		remote := args[1]
		scpValue, err := ParseScpValue(remote)
		if err != nil {
			fmt.Println("parse:"+remote+", ", err)
			return
		}

		nodes := []*sshwctl.Node{{
			Host: scpValue.Host,
			User: scpValue.User,
			Scps: []*sshwctl.NodeCp{
				{
					Src: src,
					Tgt: scpValue.Path,
				},
			},
		}}
		if err := sshwctl.InitNodes(nodes); err != nil {
			fmt.Println(err)
			return
		}
		if err := ExecNode(nodes[0]); err != nil {
			fmt.Println(err)
		}
	},
}
