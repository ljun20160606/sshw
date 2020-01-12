package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// first backup filename, and then replace it with src
func backupAndReplaceFile(filename string, src io.Reader) error {
	dstFile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil && !os.IsExist(err) {
		return err
	}
	defer dstFile.Close()
	fmt.Println("backup file " + filename)
	basename := filepath.Base(filename)
	tempFile, err := ioutil.TempFile("", "bak"+basename)
	if err != nil {
		return err
	}
	defer tempFile.Close()
	if _, err := io.Copy(tempFile, dstFile); err != nil {
		return err
	}

	fmt.Println("backup name " + tempFile.Name())
	// size 0
	dstFile.Truncate(0)
	// offset 0
	dstFile.Seek(0, 0)
	if _, err = io.Copy(dstFile, src); err != nil {
		return err
	}
	return nil
}
