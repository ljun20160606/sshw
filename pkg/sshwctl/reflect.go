package sshwctl

import (
	"errors"
	"reflect"
)

// if t.kind() in array or slice, name is $index
// return true, if would skip deepSearching type like slice or array or map
type ValueSolver func(k string, t reflect.Type, v reflect.Value, structField *reflect.StructField) (skipSearch bool)

const (
	WalkIndexFlag = "$index"
	WalkOtherFlag = "$other"
)

func WalkInterface(v reflect.Value, walked bool, solver ValueSolver) error {
	t := v.Type()
	var realT reflect.Type
	var realV reflect.Value
	if t.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
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
			if skip := solver(field.Name, value.Type(), value, &field); skip {
				continue
			}
			if err := WalkInterface(value, true, solver); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for num := 0; num < realV.Len(); num++ {
			index := realV.Index(num)
			if skip := solver(WalkIndexFlag, index.Type(), index, nil); skip {
				continue
			}
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
			if skip := solver(key.Interface().(string), value.Type(), value, nil); skip {
				continue
			}
			if err := WalkInterface(value, true, solver); err != nil {
				return err
			}
		}
	default:
		if !walked {
			if skip := solver(WalkOtherFlag, realT, realV, nil); skip {
				return nil
			}
		}
	}
	return nil
}

// search not empty field in v
// if key in ignoreKeys, skip validation
func FieldsNotEmpty(v interface{}, ignoreKeys []string) ([]string, error) {
	var notEmptyNames []string
	if err := WalkInterface(reflect.ValueOf(v), false, func(k string, t reflect.Type, v reflect.Value, structField *reflect.StructField) (skip bool) {
		for i := range ignoreKeys {
			if ignoreKeys[i] == k {
				return true
			}
		}

		if structField != nil {
			if structField.Tag.Get("yaml") == "-" || structField.Tag.Get("json") == "-" {
				return true
			}
		}

		skipFunc := func() bool {
			switch t.Kind() {
			case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
				return true
			}
			return false
		}

		if !FieldEmpty(t, v) {
			notEmptyNames = append(notEmptyNames, k)
			return skipFunc()
		}
		return skipFunc()
	}); err != nil {
		return nil, err
	}
	return notEmptyNames, nil
}

func FieldEmpty(t reflect.Type, v reflect.Value) bool {
	switch t.Kind() {
	case reflect.String:
		if v.String() == "" {
			return true
		}
	case reflect.Array, reflect.Slice, reflect.Map:
		if v.Len() == 0 {
			return true
		}
	case reflect.Ptr:
		if v.IsNil() {
			return true
		}
		elem := v.Elem()
		return FieldEmpty(v.Elem().Type(), elem)
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		if v.Int() == 0 {
			return true
		}
	case reflect.Bool:
		if !v.Bool() {
			return true
		}
	default:
		// 其他类型默认算不为空
	}
	return false
}
