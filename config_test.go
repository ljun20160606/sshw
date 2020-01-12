package sshw

import (
	"github.com/stretchr/testify/assert"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeNodes(tt.args.dstPtr, tt.args.src)
			ast.EqualValues(tt.args.expect, *tt.args.dstPtr)
		})
	}
}
