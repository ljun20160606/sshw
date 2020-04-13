package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
)

func Test_versionNewCmd(t *testing.T) {
	t.Skip()
	ast := assert.New(t)
	showVersionCmd := exec.Command("sshw", "version", "new")
	output, err := showVersionCmd.Output()
	ast.Nil(err)
	fmt.Println(string(output))
}
