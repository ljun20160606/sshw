package main

import (
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"github.com/spf13/cobra"
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
	githubRepository = &sshwctl.GithubRepository{
		Url:      "https://github.com",
		Username: "ljun20160606",
		Name:     sshwctl.ApplicationName,
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
		binaryFile, err := sshwctl.ExtractBinary(tempFile.Name(), false)
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
		binaryFile.Seek(0, 0)
		if err := backupAndReplaceFile(path, binaryFile); err != nil {
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
		showVersion()
		fmt.Println("latest version:", versionMeta.Version)
	},
}
