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
		ast.True(UserIdRsaIsNotExist())
	} else {
		ast.Nil(err)
	}
}
