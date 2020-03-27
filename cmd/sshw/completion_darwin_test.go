package main

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_genCompletion(t *testing.T) {
	ast := assert.New(t)
	//genFishCompletion()
	_, err := os.Stat(fishCompletionPath)
	ast.Nil(err)
}