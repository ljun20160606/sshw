package main

import (
	"bytes"
	"fmt"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"net/url"
)

func init() {
	rootCmd.AddCommand(mergeCmd)
}

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "merge config by name",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filename := rootCmd.PersistentFlags().Lookup("filename").Value.String()
		if _, err := url.ParseRequestURI(filename); err == nil {
			fmt.Println("Can not merge config to remote config.")
			return
		}
		pathname, dstNodes, err := sshwctl.LoadYamlConfig(filename)
		if err != nil {
			fmt.Println(err)
			return
		}
		_, srcNodes, err := sshwctl.LoadYamlConfig(args[0])
		if err != nil {
			fmt.Println("load expected yaml", err)
			return
		}
		sshwctl.MergeNodes(&dstNodes, srcNodes)

		mergedNodes, err := yaml.Marshal(dstNodes)
		if err != nil {
			fmt.Println("marshal merged config", err)
			return
		}
		if err := backupAndReplaceFile(pathname, bytes.NewReader(mergedNodes)); err != nil {
			fmt.Println("replace config", err)
			return
		}
		fmt.Println("Merge finished")
	},
}
