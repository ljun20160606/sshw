package main

import (
	"fmt"
	"github.com/ljun20160606/sshw/pkg/multiplex"
	"github.com/spf13/cobra"
	"os"
	"strconv"
)

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverStatusCmd)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "manage server of multiplexing session",
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "run a server for multiplexing session",
	Run: func(cmd *cobra.Command, args []string) {
		newServer := multiplex.NewServer()
		newServer.Handler = multiplex.NewMasterHandler()
		if err := newServer.ListenAndServe(); err != nil {
			fmt.Println(err)
		}
	},
}

func getProcess() *os.Process{
	pid, has := ReadPid()
	if !has {
		fmt.Println("no pid in " + SSHWPidPath)
		os.Exit(0)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("can not find process by pid read from " + SSHWPidPath)
		os.Exit(0)
	}
	return process
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "server status",
	Run: func(cmd *cobra.Command, args []string) {
		process := getProcess()
		if multiplex.IsRunning() {
			fmt.Println("server is running, pid is " + strconv.Itoa(process.Pid))
			return
		}
		fmt.Println("server is not running")
	},
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop server",
	Run: func(cmd *cobra.Command, args []string) {
		if !multiplex.IsRunning() {
			fmt.Println("server is not running")
			return
		}
		process := getProcess()
		fmt.Println("stopping server " + strconv.Itoa(process.Pid))
		_ = process.Kill()
		fmt.Println("finish")
	},
}
