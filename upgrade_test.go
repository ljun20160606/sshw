package sshw

import (
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"testing"
)

var g = &GithubRepository{
	Url:      "https://github.com",
	Username: "ljun20160606",
	Name:     ApplicationName,
}

func TestGithubRepository_LatestVersions(t *testing.T) {
	ast := assert.New(t)

	v, err := g.LatestVersion()
	ast.Nil(err)

	newVersion, err := version.NewVersion(v.Version)
	ast.Nil(err)
	constraint, err := version.NewConstraint(">= v1.4.1")
	ast.Nil(err)
	check := constraint.Check(newVersion)
	ast.True(check)
}

func TestGithubRepository_Download(t *testing.T) {
	ast := assert.New(t)
	meta, err := g.LatestVersion()
	ast.Nil(err)
	_, err = g.Download(meta)
	ast.Nil(err)
}

func TestLocalExtraBinary(t *testing.T) {
	ast := assert.New(t)
	f := "/var/folders/lt/y_vkfnbd5ll7_dn4s1rknkz40000gn/T/sshw-v1.4.1-darwin-osx-amd64.zip274860995"
	file, err := ExtractBinary(f, true)
	ast.Nil(err)
	fmt.Println(file.Name())
}
