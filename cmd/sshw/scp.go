package main

import (
	"fmt"
	"github.com/ljun20160606/sshw/pkg/language"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"github.com/spf13/cobra"
)

var isLocal bool

func init() {
	scpCmd.Flags().BoolVarP(&isLocal, "local", "l", false, "do not control-master")
	rootCmd.AddCommand(scpCmd)
}

var scpCmd = &cobra.Command{
	Use:     "scp",
	Short:   "like scp",
	Example: "sshw scp file user@host:",
	Args:    cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		srcParam := args[0]
		tgtParam := args[1]
		isRemote := !isLocal

		var host, user string
		var src, tgt string
		var isReceive bool

		if srcValue, err := language.ParseScpDestination(srcParam); err == nil {
			isReceive = true
			host = srcValue.Host
			user = srcValue.User
			src = srcValue.Path
			tgt = tgtParam
		} else {
			if tgtValue, err := language.ParseScpDestination(tgtParam); err != nil {
				fmt.Println(err)
				return
			} else {
				host = tgtValue.Host
				user = tgtValue.User
				src = srcParam
				tgt = tgtValue.Path
			}
		}

		nodes := []*sshwctl.Node{{
			Host: host,
			User: user,
			Scps: []*sshwctl.NodeCp{
				{
					Src:       src,
					Tgt:       tgt,
					IsReceive: isReceive,
				},
			},
			ControlMaster: &isRemote,
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
