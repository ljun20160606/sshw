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
	versionCmd.AddCommand(versionNewCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show version",
	Run: func(cmd *cobra.Command, args []string) {
		showVersion()
	},
}

var (
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
			fmt.Println(err)
			return
		}
		remoteVersion, err := version.NewVersion(versionMeta.Version)
		if err != nil {
			fmt.Println(err)
			return
		}
		localVersion, err := version.NewVersion(Version)
		if err != nil {
			fmt.Println(err)
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
			fmt.Println(err)
			return
		}
		binaryFile, err := sshwctl.ExtractBinary(tempFile.Name(), false)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer binaryFile.Close()
		lookPath, err := exec.LookPath(os.Args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("Exec path ", lookPath)
		fmt.Println("Upgrade started")
		binaryFile.Seek(0, 0)
		if err := backupAndReplaceFile(lookPath, binaryFile); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("Upgrade finished")

		// exec: sshw -v to show new version
		showVersionCmd := exec.Command(lookPath, "version", "new")
		showVersionCmd.Stdin = os.Stdin
		showVersionCmd.Stdout = os.Stdout
		showVersionCmd.Stderr = os.Stderr
		if err := showVersionCmd.Start(); err != nil {
			fmt.Println(err)
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
			fmt.Println(err)
			return
		}
		showVersion()
		fmt.Println("latest version:", versionMeta.Version)
	},
}

func showVersion() {
	fmt.Println("sshw - ssh client wrapper for automatic login")
	fmt.Println("version:", Version)
}

var versionNewCmd = &cobra.Command{
	Use:    "new",
	Short:  "Show version new",
	Run: func(cmd *cobra.Command, args []string) {
		showVersion()
		fmt.Println("It's recommended to stop running sever after upgrade:\n    sshw server stop")
		fmt.Println("Get completion:\n    sshw completion")
	},
}
