package sshwctl

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"os"
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
		//{
		//	name: "add",
		//	args: args{
		//		dstPtr: &([]*Node{
		//			{
		//				Name:  "foo",
		//				Alias: "foo",
		//			},
		//		}),
		//		src: []*Node{
		//			{
		//				Name:  "bar",
		//				Alias: "bar",
		//			},
		//		},
		//		expect: []*Node{
		//			{
		//				Name:  "foo",
		//				Alias: "foo",
		//			},
		//			{
		//				Name:  "bar",
		//				Alias: "bar",
		//			},
		//		},
		//	},
		//},
		//{
		//	name: "override",
		//	args: args{
		//		dstPtr: &([]*Node{
		//			{
		//				Name:  "foo",
		//				Alias: "foo",
		//			},
		//		}),
		//		src: []*Node{
		//			{
		//				Name:  "foo",
		//				Alias: "bar",
		//			},
		//		},
		//		expect: []*Node{
		//			{
		//				Name:  "foo",
		//				Alias: "bar",
		//			},
		//		},
		//	},
		//},
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

	type args struct {
		p string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "relative",
			args: args{
				p: "./",
			},
			want: wd,
		},
		{
			name: "blank",
			args: args{
				p: "",
			},
			want: "",
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
