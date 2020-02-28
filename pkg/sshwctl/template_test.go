package sshwctl

import (
	"os"
	"reflect"
	"testing"
)

func TestParseSshwTemplate(t *testing.T) {
	type args struct {
		src  string
		pre  func()
		post func()
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "anyenv",
			args: args{
				src: "${ANYUSER}",
				pre: func() {
					_ = os.Setenv("ANYUSER", "tester")
				},
				post: func() {
					_ = os.Setenv("ANYUSER", "")
				},
			},
			want: "tester",
		},
		{
			name: "multiParams-useFirst",
			args: args{
				src: "${FIRST,SECOND}",
				pre: func() {
					_ = os.Setenv("FIRST", "FIRST")
				},
				post: func() {
					_ = os.Setenv("FIRST", "")
				},
			},
			want: "FIRST",
		},
		{
			name: "multiParams-useSecond",
			args: args{
				src: "${FIRST,SECOND}",
				pre: func() {
					_ = os.Setenv("SECOND", "SECOND")
				},
				post: func() {
					_ = os.Setenv("SECOND", "")
				},
			},
			want: "SECOND",
		},
		{
			name: "escape",
			args: args{src: "\\${ANYUSER}"},
			want: "${ANYUSER}",
		},
		{
			name: "defaultValue",
			args: args{src: "${ANYUSER:defaultUser}"},
			want: "defaultUser",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.pre != nil {
				tt.args.pre()
			}
			if got := ParseSshwTemplate(tt.args.src).Execute(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSshwTemplate() = %v, want %v", got, tt.want)
			}
			if tt.args.post != nil {
				tt.args.post()
			}
		})
	}
}
