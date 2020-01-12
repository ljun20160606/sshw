package main

import (
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/ljun20160606/sshw"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
)

func init() {
	rootCmd.AddCommand(latestCmd, upgradeCmd, versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show version",
	Run: func(cmd *cobra.Command, args []string) {
		showVersion()
	},
}

func showVersion() {
	fmt.Println("sshw - ssh client wrapper for automatic login")
	fmt.Println("version:", Version)
}

var (
	originalWD, _    = os.Getwd()
	githubRepository = &sshw.GithubRepository{
		Url:      "https://github.com",
		Username: "ljun20160606",
		Name:     sshw.ApplicationName,
	}
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "upgrade sshw",
	Run: func(cmd *cobra.Command, args []string) {
		versionMeta, err := githubRepository.LatestVersion()
		if err != nil {
			log.Error(err)
			return
		}
		remoteVersion, err := version.NewVersion(versionMeta.Version)
		if err != nil {
			log.Error(err)
			return
		}
		localVersion, err := version.NewVersion(Version)
		if err != nil {
			log.Error(err)
			return
		}
		if localVersion.Compare(remoteVersion) >= 0 {
			fmt.Println("localVersion: ", Version)
			fmt.Println("remoteVersion: ", versionMeta.Version)
			fmt.Println("local is latest.")
			return
		}
		tempFile, err := githubRepository.Download(versionMeta)
		if err != nil {
			log.Error(err)
			return
		}
		binaryFile, err := sshw.ExtractBinary(tempFile.Name(), false)
		if err != nil {
			log.Error(err)
			return
		}
		defer binaryFile.Close()
		path, err := exec.LookPath(os.Args[0])
		if err != nil {
			log.Error(err)
			return
		}
		fmt.Println("Exec path ", path)
		fmt.Println("Upgrade started")
		execFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil && !os.IsExist(err) {
			log.Error(err)
			return
		}
		defer execFile.Close()
		// size 0
		execFile.Truncate(0)
		// offset 0
		execFile.Seek(0, 0)
		binaryFile.Seek(0, 0)
		if _, err = io.Copy(execFile, binaryFile); err != nil {
			log.Error(err)
			return
		}
		fmt.Println("Upgrade finished")

		// exec: sshw -v to show new version
		allFiles := append([]*os.File{os.Stdin, os.Stdout, os.Stderr})
		_, err = os.StartProcess(path, []string{path, "-v"}, &os.ProcAttr{
			Dir:   originalWD,
			Files: allFiles,
		})
		if err != nil {
			log.Error(err)
			return
		}
	},
}

var latestCmd = &cobra.Command{
	Use:   "latest",
	Short: "get latest version in remote",
	Run: func(cmd *cobra.Command, args []string) {
		versionMeta, err := githubRepository.LatestVersion()
		if err != nil {
			log.Error(err)
			return
		}
		fmt.Println("latest version:", versionMeta.Version)
	},
}
