package main

import (
	"fmt"
	"github.com/ljun20160606/sshw/pkg/multiplex"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serverCmd)
}

var serverCmd = &cobra.Command{
	Use:    "server",
	Short:  "run a server for multiplexing session",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		newServer := multiplex.NewServer()
		newServer.Handler = multiplex.NewMasterHandler()
		if err := newServer.ListenAndServe(); err != nil {
			fmt.Println(err)
		}
	},
}
