package main

import (
	"reflect"
	"testing"
)

func TestParseScpValue(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name    string
		args    args
		want    *ScpValueGrammar
		wantErr bool
	}{
		{
			name:    ": and path",
			args:    args{input: ":path"},
			wantErr: true,
		},
		{
			name:    "user and @",
			args:    args{input: "user@"},
			wantErr: true,
		},
		{
			name:    "user and @path",
			args:    args{input: "user@path"},
			wantErr: true,
		},
		{
			name: "host and :",
			args: args{input: "host:"},
			want: &ScpValueGrammar{
				Host: "host",
			},
		},
		{
			name:    "only host",
			args:    args{input: "host"},
			wantErr: true,
		},
		{
			name: "host and path",
			args: args{input: "host:path"},
			want: &ScpValueGrammar{
				Host: "host",
				Path: "path",
			},
		},
		{
			name: "user and host and path",
			args: args{input: "user@host:path"},
			want: &ScpValueGrammar{
				User: "user",
				Host: "host",
				Path: "path",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseScpValue(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseScpValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseScpValue() got = %v, want %v", got, tt.want)
			}
		})
	}
}
