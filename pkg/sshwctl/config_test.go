package sshwctl

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"testing"
)

func TestMergeNodes(t *testing.T) {
	ast := assert.New(t)
	type args struct {
		dstPtr *[]*Node
		src    []*Node
		expect []*Node
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "add",
			args: args{
				dstPtr: &([]*Node{
					{
						Name:  "foo",
						Alias: "foo",
					},
				}),
				src: []*Node{
					{
						Name:  "bar",
						Alias: "bar",
					},
				},
				expect: []*Node{
					{
						Name:  "foo",
						Alias: "foo",
					},
					{
						Name:  "bar",
						Alias: "bar",
					},
				},
			},
		},
		{
			name: "override",
			args: args{
				dstPtr: &([]*Node{
					{
						Name:  "foo",
						Alias: "foo",
					},
				}),
				src: []*Node{
					{
						Name:  "foo",
						Alias: "bar",
					},
				},
				expect: []*Node{
					{
						Name:  "foo",
						Alias: "bar",
					},
				},
			},
		},
		{
			name: "bookmark",
			args: args{
				dstPtr: &([]*Node{
					{
						Name: "foo",
						Children: []*Node{
							{
								Name: "bar",
							},
						},
					},
				}),
				src: []*Node{
					{
						Name: "foo",
						Children: []*Node{
							{
								Name: "car",
							},
						},
					},
				},
				expect: []*Node{
					{
						Name: "foo",
						Children: []*Node{
							{
								Name: "bar",
							},
							{
								Name: "car",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected, _ := yaml.Marshal(tt.args.expect)
			MergeNodes(tt.args.dstPtr, tt.args.src)
			dst, _ := yaml.Marshal(*tt.args.dstPtr)
			ast.EqualValues(string(expected), string(dst))
		})
	}
}

func TestIsBookmark(t *testing.T) {
	type args struct {
		n *Node
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "one",
			args: args{n: &Node{
				Name: "Foo",
				Children: []*Node{
					{
						Name:  "foo",
						Alias: "foo",
						User:  "user",
					},
				},
			}},
			want: true,
		},
		{
			name: "two",
			args: args{n: &Node{
				Name:  "Foo",
				Alias: "Foo",
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBookmark(tt.args.n); got != tt.want {
				t.Errorf("IsBookmark() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAbsPath(t *testing.T) {
	wd, _ := os.Getwd()
	current, _ := user.Current()

	type args struct {
		p string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "blank",
			args: args{
				p: "",
			},
			want: "",
		},
		{
			name: "relative",
			args: args{
				p: "./",
			},
			want: wd,
		},
		{
			name: "home",
			args: args{
				p: "~",
			},
			want: current.HomeDir,
		},
		{
			name: "wild pattern",
			args: args{
				p: "./config_test.*",
			},
			want: wd + "/config_test.go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AbsPath(tt.args.p); got != tt.want {
				t.Errorf("AbsPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	ast := assert.New(t)

	config, err := LoadConfig([]byte(`
name: foo
---
name: bar
---
- name: zoo
`))
	ast.Nil(err)
	ast.Len(config, 3)
	ast.Equal(config[0].Name, "foo")
	ast.Equal(config[1].Name, "bar")
	ast.Equal(config[2].Name, "zoo")

	_, err = LoadConfig([]byte(`"`))
	ast.NotNil(err)
}

func TestReadRemoteConfig(t *testing.T) {
	ast := assert.New(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("- name: Hello"))
	}))
	defer ts.Close()

	pathname, nodes, err := LoadYamlConfig(ts.URL)
	if !ast.Nil(err) {
		return
	}
	if !ast.Equal(ts.URL, pathname) {
		return
	}
	if ast.Len(nodes, 1) {
		ast.Equal(nodes[0].Name, "Hello")
	}
}

func TestMergeSshConfig(t *testing.T) {
	ast := assert.New(t)

	nodes := []*Node{
		{
			Host: "daily",
		},
	}
	sshNodes := []*Node{
		{
			Name: "daily",
			Host: "sshw.com",
			User: "developer",
			Port: 21,
		},
	}
	if err := MergeSshConfig(nodes, sshNodes); !ast.Nil(err) {
		return
	}

	ast.Equal(sshNodes[0].Host, nodes[0].Host)
	ast.Equal(sshNodes[0].User, nodes[0].User)
	ast.Equal(sshNodes[0].Port, nodes[0].Port)
}
