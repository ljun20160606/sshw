package sshw

import (
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
)

type Repository interface {
	// get remote version
	LatestVersion() (string, error)

	// download remote file to specified path
	Download(versionMeta *VersionMeta) (*os.File, error)
}

var _ Repository = &GithubRepository{}

type GithubRepository struct {
	Url      string
	Username string
	Name     string
}

func (g *GithubRepository) url() string {
	return g.Url + "/" + g.Username + "/" + g.Name
}

func (g *GithubRepository) releases() string {
	return g.url() + "/releases"
}

func (g *GithubRepository) LatestVersion() (string, error) {
	meta, err := g.latestVersion()
	if err != nil {
		return "", err
	}
	return meta.Version, nil
}

func (g *GithubRepository) latestVersion() (*VersionMeta, error) {
	system, err := findSupportSystem()
	if err != nil {
		return nil, err
	}
	response, err := http.Get(g.releases())
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	versions := extractVersions(body)
	for i := range versions {
		versionMeta := &versions[i]
		if strings.Contains(versionMeta.Filename, system.goos()) {
			if strings.Contains(versionMeta.Filename, system.GOARCH) || strings.Contains(versionMeta.Filename, system.goarch()) {
				return versionMeta, nil
			}
		}
	}
	return nil, remoteUnsupported
}

type VersionMeta struct {
	// version
	// e.t. v1.4.1
	Version string

	// file suffix
	// e.t. sshw-v1.4.1-darwin-osx-amd64.zip
	Filename string
}

var versionsCompile = regexp.MustCompile(`download/(\S+?)/(\S+?)"`)

// To extract versions from remote repository.
func extractVersions(content []byte) []VersionMeta {
	var versions []VersionMeta
	for i := 0; i < 4; i++ {
		submatch := versionsCompile.FindSubmatch(content)
		if len(submatch) > 2 {
			versions = append(versions, VersionMeta{string(submatch[1]), string(submatch[2])})
			continue
		}
		return versions
	}

	return versions
}

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer
// interface and we can pass this into io.TeeReader() which will report progress on each
// write cycle.
type WriteCounter struct {
	Total uint64
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	wc.PrintProgress()
	return n, nil
}

func (wc WriteCounter) PrintProgress() {
	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 35))

	// Return again and print current status of download
	// We use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\rDownloading... %s complete", humanize.Bytes(wc.Total))
}

// Using version and filename to generate a remote url that is used to download file.
// Download it to file tmp. Then backup original file and replace it with Downloaded file.
func (g *GithubRepository) Download(versionMeta *VersionMeta) (*os.File, error) {
	downloadUrl := g.downloadUrl(versionMeta.Version, versionMeta.Filename)
	response, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	tempFile, err := ioutil.TempFile("", versionMeta.Filename)
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()
	fmt.Println("Download Started")
	counter := &WriteCounter{}
	_, err = io.Copy(tempFile, io.TeeReader(response.Body, counter))
	if err != nil {
		return nil, err
	}
	fmt.Print("\n")
	fmt.Println("Download Finished")
	fmt.Println("Tmp file ", tempFile.Name())

	return tempFile, nil
}

// versionName v1.4.1
// GOOS darwin
// GOARCH amd64
// return https://github.com/ljun20160606/sshw/releases/download/v1.4.1/sshw-v1.4.1-darwin-osx-amd64.zip
func (g *GithubRepository) downloadUrl(version, filename string) string {
	return g.releases() + "/download/" + version + "/" + filename
}

type RuntimeSystem struct {
	GOOS        string
	GOOSAlias   string
	GOARCH      string
	GOARCHAlias string
}

func (r *RuntimeSystem) goos() string {
	if r.GOOSAlias != "" {
		return r.GOOSAlias
	}
	return r.GOOS
}

func (r *RuntimeSystem) goarch() string {
	if r.GOARCHAlias != "" {
		return r.GOARCHAlias
	}
	return r.GOARCH
}

var supportedSystem = []RuntimeSystem{
	{"darwin", "darwin-osx", "amd64", ""},
	{"linux", "", "amd64", ""},
	{"windows", "", "amd64", "x64"},
	{"windows", "", "386", "x86"},
}

var (
	systemUnsupported = errors.New("GOOS " + runtime.GOOS + ", GOARCH " + runtime.GOARCH + " is not supported")
	remoteUnsupported = errors.New("GOOS " + runtime.GOOS + ", GOARCH " + runtime.GOARCH + " does not in remote releases")
)

func findSupportSystem() (*RuntimeSystem, error) {
	for i := range supportedSystem {
		s := supportedSystem[i]
		if runtime.GOOS == s.GOOS && runtime.GOARCH == s.GOARCH {
			return &s, nil
		}
	}
	return nil, systemUnsupported
}
