package sshw

import (
	"github.com/stretchr/testify/assert"
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

func TestWalkInterface(t *testing.T) {
	ast := assert.New(t)
	type args struct {
		v      reflect.Value
		walked bool
		solver ValueSolver
		test   func(args)
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "write name",
			args: args{
				v:      reflect.ValueOf([]*Node{{Name: "foo"}}),
				walked: false,
				solver: func(k string, t reflect.Type, v reflect.Value) {
					if t.Kind() == reflect.String && v.CanSet() {
						if k == "Name" {
							v.Set(reflect.ValueOf("bar"))
						}
					}
				},
				test: func(a args) {
					ast.NotPanics(func() {
						name := a.v.Interface().([]*Node)[0].Name
						if !reflect.DeepEqual(name, "bar") {
							t.Errorf("WalkInterface(), Name = %s, want %s", "bar", name)
						}
					})
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := WalkInterface(tt.args.v, tt.args.walked, tt.args.solver); (err != nil) != tt.wantErr {
				t.Errorf("WalkInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
