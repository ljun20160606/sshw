package main

import (
	"bytes"
	"fmt"
	"github.com/ljun20160606/sshw"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
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
		pathname, dstNodes, err := sshw.LoadYamlConfig(filename)
		if err != nil {
			log.Error("load yaml config", err)
			return
		}
		_, srcNodes, err := sshw.LoadYamlConfig(args[0])
		if err != nil {
			log.Error("load expected merge yaml config", err)
			return
		}
		sshw.MergeNodes(&dstNodes, srcNodes)

		mergedNodes, err := yaml.Marshal(dstNodes)
		if err != nil {
			log.Error("marshal merged config", err)
			return
		}
		if err := backupAndReplaceFile(pathname, bytes.NewReader(mergedNodes)); err != nil {
			log.Error("replace config", err)
			return
		}
		fmt.Println("Merge finished")
	},
}
