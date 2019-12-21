package sshw

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"text/scanner"
)

type ValueSolver func(k string, t reflect.Type, v reflect.Value)

func WalkInterface(v reflect.Value, walked bool, solver ValueSolver) error {
	t := v.Type()
	var realT reflect.Type
	var realV reflect.Value
	if t.Kind() == reflect.Ptr {
		realT = t.Elem()
		realV = v.Elem()
	} else {
		realT = t
		realV = v
	}

	switch kind := realT.Kind(); kind {
	case reflect.Struct:
		for num := 0; num < realV.NumField(); num++ {
			field := realT.Field(num)
			value := realV.Field(num)
			solver(field.Name, value.Type(), value)
			if err := WalkInterface(value, true, solver); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for num := 0; num < realV.Len(); num++ {
			index := realV.Index(num)
			solver("$index", index.Type(), index)
			if err := WalkInterface(index, true, solver); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := realV.MapRange()
		for iter.Next() {
			key := iter.Key()
			value := iter.Value()
			if key.Kind() != reflect.String {
				return errors.New("only support key of string in map")
			}
			solver(key.Interface().(string), value.Type(), value)
			if err := WalkInterface(value, true, solver); err != nil {
				return err
			}
		}
	default:
		if !walked {
			solver("$other", realT, realV)
		}
	}
	return nil
}

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
			if len(spiltColon) == 1 {
				continue
			}
			defaultValue := spiltColon[1]
			if defaultValue != "" {
				builder.WriteString(defaultValue)
				continue
			}
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
