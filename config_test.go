package sshw

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"reflect"
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
						Name:  "foo",
						Children: []*Node{
							{
								Name: "bar",
							},
						},
					},
				}),
				src: []*Node{
					{
						Name:  "foo",
						Children: []*Node{
							{
								Name: "car",
							},
						},
					},
				},
				expect: []*Node{
					{
						Name:  "foo",
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
			got, err := FieldsEmpty(tt.args.v, tt.args.ignoreKeys)
			if (err != nil) != tt.wantErr {
				t.Errorf("FieldsEmpty() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FieldsEmpty() got = %v, want %v", got, tt.want)
			}
		})
	}
}
