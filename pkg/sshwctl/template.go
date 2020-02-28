package sshwctl

import (
	"bytes"
	"os"
	"strings"
	"text/scanner"
)

const (
	TypeStr   = "str"
	TypeParam = "param"
)

type CustomTemplate struct {
	Templates []*TemplateNode
}

func (c *CustomTemplate) Execute() string {
	builder := strings.Builder{}
TEMPLATE:
	for i := range c.Templates {
		templateNode := c.Templates[i]
		switch templateNode.Type {
		case TypeStr:
			builder.WriteString(templateNode.Value)
		case TypeParam:
			s := templateNode.Value[2 : len(templateNode.Value)-1]
			if len(s) == 0 {
				builder.WriteString(templateNode.Value)
				continue
			}
			spiltColon := strings.SplitN(s, ":", 2)
			envKeys := spiltColon[0]
			splitComma := strings.Split(envKeys, ",")
			for i := range splitComma {
				envKey := splitComma[i]
				envValue := os.Getenv(envKey)
				if envValue != "" {
					builder.WriteString(envValue)
					continue TEMPLATE
				}
			}
			if len(spiltColon) != 1 {
				defaultValue := spiltColon[1]
				if defaultValue != "" {
					builder.WriteString(defaultValue)
					continue
				}
			}
			// if and when can not find value, write origin str
			builder.WriteString(templateNode.Value)
		}
	}
	return builder.String()
}

type TemplateNode struct {
	Type  string
	Value string
}

func NewStrNode(v string) *TemplateNode {
	return &TemplateNode{
		Type:  TypeStr,
		Value: v,
	}
}

func NewParamNode(v string) *TemplateNode {
	return &TemplateNode{
		Type:  TypeParam,
		Value: v,
	}
}

// Parse string to Template
// For example:
// INPUT:
// ParseSshwTemplate("Foo${foo:a}bar")
// OUTPUT:
// []{{Type: TypeStr, Value: "Foo"}, {Type: TypeParam}, Value: "${foo:a}"}, {Type: TypeStr, Value: "bar"}}
func ParseSshwTemplate(src string) *CustomTemplate {
	if src == "" {
		return &CustomTemplate{}
	}
	var s scanner.Scanner
	s.Init(strings.NewReader(src))
	var out strings.Builder
	stack := bytes.NewBuffer(nil)
	var tree []*TemplateNode
	var flush = func() {
		if out.Len() != 0 {
			tree = append(tree, NewStrNode(out.String()))
			out.Reset()
		}
		if stack.Len() != 0 {
			tree = append(tree, NewParamNode(stack.String()))
			stack.Reset()
		}
	}
	for next := s.Next(); next != scanner.EOF; next = s.Next() {
		switch next {
		case '\\':
			if stack.Len() == 0 {
				peek := s.Peek()
				if peek == '$' {
					out.WriteRune(s.Next())
				} else {
					out.WriteRune(next)
				}
				continue
			}
			stack.WriteRune(next)
		case '$':
			if stack.Len() == 0 {
				peek := s.Peek()
				if peek == '{' {
					stack.WriteRune(next)
					stack.WriteRune(s.Next())
					continue
				}
				out.WriteRune(next)
				continue
			}
			stack.WriteRune(next)
		case '}':
			if stack.Len() == 0 {
				out.WriteRune(next)
				continue
			}
			stack.WriteRune(next)
			flush()
		default:
			if stack.Len() != 0 {
				stack.WriteRune(next)
			} else {
				out.WriteRune(next)
			}
		}
	}
	flush()
	return &CustomTemplate{Templates: tree}
}
