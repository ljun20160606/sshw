package sshwctl

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

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
				solver: func(k string, t reflect.Type, v reflect.Value, structField *reflect.StructField) (stop bool) {
					if t.Kind() == reflect.String && v.CanSet() {
						if k == "Name" {
							v.Set(reflect.ValueOf("bar"))
						}
					}
					return
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

func TestFieldsEmpty(t *testing.T) {
	type args struct {
		v          interface{}
		ignoreKeys []string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "one",
			args: args{
				v: struct {
					Number int8
				}{
					Number: 1,
				},
				ignoreKeys: nil,
			},
			want:    []string{"Number"},
			wantErr: false,
		},
		{
			name: "two",
			args: args{
				v: struct {
					Name   string
					Number int
				}{
					Name:   "foo",
					Number: 1,
				},
				ignoreKeys: []string{"Name", "Number"},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FieldsNotEmpty(tt.args.v, tt.args.ignoreKeys)
			if (err != nil) != tt.wantErr {
				t.Errorf("FieldsNotEmpty() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FieldsNotEmpty() got = %v, want %v", got, tt.want)
			}
		})
	}
}
