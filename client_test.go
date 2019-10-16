package sshw

import "testing"

func Test_execs(t *testing.T) {
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
			if err := execs(tt.args.execs); (err != nil) != tt.wantErr {
				t.Errorf("execs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}