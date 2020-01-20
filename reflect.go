package sshw

import (
	"errors"
	"reflect"
)

type ValueSolver func(k string, t reflect.Type, v reflect.Value) (stop bool)

const WalkIndexFlag = "$index"

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
			if stop := solver(field.Name, value.Type(), value); stop {
				return nil
			}
			if err := WalkInterface(value, true, solver); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for num := 0; num < realV.Len(); num++ {
			index := realV.Index(num)
			if stop := solver(WalkIndexFlag, index.Type(), index); stop {
				return nil
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
			if stop := solver(key.Interface().(string), value.Type(), value); stop {
				return nil
			}
			if err := WalkInterface(value, true, solver); err != nil {
				return err
			}
		}
	default:
		if !walked {
			if stop := solver("$other", realT, realV); stop {
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
	if err := WalkInterface(reflect.ValueOf(v), false, func(k string, t reflect.Type, v reflect.Value) (stop bool) {
		if k == WalkIndexFlag {
			return
		}
		for i := range ignoreKeys {
			if ignoreKeys[i] == k {
				return
			}
		}
		if !FieldEmpty(t, v) {
			notEmptyNames = append(notEmptyNames, k)
			return
		}
		return
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
