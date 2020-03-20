package main

import (
	"fmt"
	"testing"
)

func TestReadPid(t *testing.T) {
	pid, _ := ReadPid()
	fmt.Println(pid)
}
