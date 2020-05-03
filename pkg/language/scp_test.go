package language

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
		want    *ScpDestinationGrammar
		wantErr bool
	}{
		{
			name: "string_user",
			args: args{input: `"user"@localhost:test.txt`},
			want: &ScpDestinationGrammar{
				User: "user",
				Host: "localhost",
				Path: "test.txt",
			},
		},
		{
			name: "path_contain_dot",
			args: args{input: "localhost:test.txt"},
			want: &ScpDestinationGrammar{
				Host: "localhost",
				Path: "test.txt",
			},
		},
		{
			name:    ":_and_path",
			args:    args{input: ":path"},
			wantErr: true,
		},
		{
			name:    "user_and_@",
			args:    args{input: "user@"},
			wantErr: true,
		},
		{
			name:    "user_and_@path",
			args:    args{input: "user@path"},
			wantErr: true,
		},
		{
			name: "host_and_:",
			args: args{input: "host:"},
			want: &ScpDestinationGrammar{
				Host: "host",
			},
		},
		{
			name:    "only_host",
			args:    args{input: "host"},
			wantErr: true,
		},
		{
			name: "host_and_path",
			args: args{input: "host:path"},
			want: &ScpDestinationGrammar{
				Host: "host",
				Path: "path",
			},
		},
		{
			name: "user_and_host_and_path",
			args: args{input: "user@host:path"},
			want: &ScpDestinationGrammar{
				User: "user",
				Host: "host",
				Path: "path",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseScpDestination(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseScpDestination() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseScpDestination() got = %v, want %v", got, tt.want)
			}
		})
	}
}
