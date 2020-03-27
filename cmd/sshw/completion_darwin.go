// +build darwin

package main

import (
	"bytes"
	"fmt"
	cobra_extension "github.com/ljun20160606/sshw/pkg/cobra-extension"
	"github.com/ljun20160606/sshw/pkg/sshwctl"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	rootCmd.AddCommand(completion)
	completion.AddCommand(completionUninstall, completionOutput)
}

var completion = &cobra.Command{
	Use:   "completion",
	Short: "Generates completion scripts",
	Run: func(cmd *cobra.Command, args []string) {
		generator := GetCompletion()
		if generator == nil {
			fmt.Println("current shell is not supported")
			return
		}
		if err := generator.Gen(); err != nil {
			fmt.Println(err)
		}
	},
}

var completionOutput = &cobra.Command{
	Use:   "output",
	Short: "Output completion scripts",
	Run: func(cmd *cobra.Command, args []string) {
		generator := GetCompletion()
		if generator == nil {
			fmt.Println("current shell is not supported")
			return
		}
		generator.Output()
	},
}

var completionUninstall = &cobra.Command{
	Use:   "rm",
	Short: "Remove completion scripts",
	Run: func(cmd *cobra.Command, args []string) {
		generator := GetCompletion()
		if generator == nil {
			fmt.Println("current shell is not supported")
			return
		}
		if err := generator.Remove(); err != nil {
			fmt.Println(err)
		}
	},
}

const (
	fishCompletionDir = ".config/fish/completions"
)

var (
	homedir, _         = os.UserHomeDir()
	fishCompletionPath = filepath.Join(homedir, fishCompletionDir, sshwctl.ApplicationName+".fish")
	dotZshrc           = filepath.Join(homedir, ".zshrc")
)

func GetCompletion() CompletionGenerator {
	shell := sshwctl.Shell()
	if strings.HasSuffix(shell, "fish") {
		return &FishCompletionGenerator{}
	}
	if strings.HasSuffix(shell, "zsh") {
		return &ZshCompletionGenerator{}
	}
	return nil
}

type CompletionGenerator interface {
	Gen() error
	Output()
	Remove() error
}

type FishCompletionGenerator struct {
}

func (f *FishCompletionGenerator) Output() {
	_ = cobra_extension.GenFishCompletion(rootCmd, os.Stdout)
}

func (f *FishCompletionGenerator) Gen() error {
	fmt.Println("Generate sshw.fish to " + fishCompletionPath)
	outFile, err := os.Create(fishCompletionPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	if err := cobra_extension.GenFishCompletion(rootCmd, outFile); err != nil {
		return err
	}
	return nil
}

func (f *FishCompletionGenerator) Remove() error {
	fmt.Println("Remove sshw.fish from " + fishCompletionPath)
	if err := os.Remove(fishCompletionPath); err != nil {
		return err
	}
	return nil
}

const (
	zshCompletionBind         = "source <(env SHELL=zsh sshw completion output)"
	zshCompletionBindTemplate = "\n# Generated by sshw, do not change it.\n" + zshCompletionBind
)

type ZshCompletionGenerator struct {
}

func (z *ZshCompletionGenerator) Output() {
	_ = cobra_extension.GenZshCompletion(rootCmd, os.Stdout)
}

func (z *ZshCompletionGenerator) Gen() error {
	fmt.Println("Add `" + zshCompletionBind + "` to " + dotZshrc)
	fileBytes, err := ioutil.ReadFile(dotZshrc)
	if err != nil {
		return err
	}

	// has been add
	if bytes.Contains(fileBytes, []byte(zshCompletionBind)) {
		return nil
	}

	outPut := bytes.NewBuffer(fileBytes)
	outPut.WriteString(zshCompletionBindTemplate)

	if err := ioutil.WriteFile(dotZshrc, outPut.Bytes(), 0644); err != nil {
		return err
	}
	return nil
}

func (z *ZshCompletionGenerator) Remove() error {
	fmt.Println("Remove `" + zshCompletionBind + "` from " + dotZshrc)
	fileBytes, err := ioutil.ReadFile(dotZshrc)
	if err != nil {
		return err
	}
	// has been removed
	if !bytes.Contains(fileBytes, []byte(zshCompletionBind)) {
		return nil
	}

	all := bytes.ReplaceAll(fileBytes, []byte(zshCompletionBindTemplate), []byte(""))
	if err := ioutil.WriteFile(dotZshrc, all, 0644); err != nil {
		return err
	}
	return nil
}
