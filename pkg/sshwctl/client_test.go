package sshwctl

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_execs(t *testing.T) {
	t.SkipNow()
	type args struct {
		execs []*NodeExec
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "scp *",
			args:    args{execs: []*NodeExec{{Cmd: "scp ./wild-* test:"}}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := execs(tt.args.execs, os.Stdin, os.Stdout); (err != nil) != tt.wantErr {
				t.Errorf("execs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAutoSSHAgent(t *testing.T) {
	ast := assert.New(t)
	if err := AutoSSHAgent(); err != nil {
		if !UserIdRsaIsNotExist() {
			ast.Nil(err)
		}
	} else {
		ast.Nil(err)
	}
}

func Test_execsVar(t *testing.T) {
	ast := assert.New(t)
	envName := "sshw_number"
	nodes := []*NodeExec{
		{
			Cmd: "echo 1",
			Var: envName,
		},
	}

	_, err := execs(nodes, os.Stdin, os.Stdout)
	ast.Nil(err)

	envValue := os.Getenv(envName)
	ast.Equal("1", envValue)
}

func Test_parseFileName(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "with filename",
			args: args{path: "./test/test.txt"},
			want: "test.txt",
		},
		{
			name: "absolute filename",
			args: args{path: "/test/test.txt"},
			want: "test.txt",
		},
		{
			name: "empty filename",
			args: args{path: "./test/"},
			want: "",
		},
		{
			name: "only dot",
			args: args{path: "."},
			want: "",
		},
		{
			name: "only relative local",
			args: args{path: "./"},
			want: "",
		},
		{
			name: "empty",
			args: args{path: ""},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseFileName(tt.args.path); got != tt.want {
				t.Errorf("parseFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}
