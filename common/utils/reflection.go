package utils

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/samber/lo"
)

// type opensearchMappingBody struct {
// 	Properties map[string]map[string]string `json:"properties"`
// }

func GetField[R any](fieldName string, caller any) R {
	ref := reflect.ValueOf(caller)
	fieldRef := ref.FieldByName(fieldName)
	if fieldRef.IsNil() {
		return lo.Empty[R]()
	}
	return fieldRef.Interface().(R)
}

func SetField(obj any, name string, value any) error {
	// Fetch the field reflect.Value
	structValue := reflect.ValueOf(obj).Elem()
	structFieldValue := structValue.FieldByName(name)

	if !structFieldValue.IsValid() {
		return fmt.Errorf("no such field: %s in obj", name)
	}

	if !structFieldValue.CanSet() {
		return fmt.Errorf("cannot set %s field value", name)
	}

	structFieldType := structFieldValue.Type()
	val := reflect.ValueOf(value)
	if !val.Type().AssignableTo(structFieldType) {
		invalidTypeError := errors.New("provided value type not assignable to obj field type")
		return invalidTypeError
	}

	structFieldValue.Set(val)
	return nil
}

func CallFunctionWithValue[R any](fn any, args ...any) R {
	fnRef := reflect.ValueOf(fn)
	argsVal := lo.Map(args, func(arg any, _ int) reflect.Value {
		return reflect.ValueOf(arg)
	})
	res := fnRef.Call(argsVal)
	if res[0].IsNil() {
		return lo.Empty[R]()
	}
	return res[0].Interface().(R)
}

func CallFunctionWithError[R any](fn any, args ...any) (R, error) {
	fnRef := reflect.ValueOf(fn)
	argsVal := lo.Map(args, func(arg any, _ int) reflect.Value {
		return reflect.ValueOf(arg)
	})
	res := fnRef.Call(argsVal)
	if res[1].IsNil() {
		return res[0].Interface().(R), nil
	}
	return lo.Empty[R](), res[1].Interface().(error)
}

func CallMethod(methodName string, caller any, args ...any) {
	method := reflect.ValueOf(caller).MethodByName(methodName)
	argsVal := lo.Map(args, func(arg any, _ int) reflect.Value {
		return reflect.ValueOf(arg)
	})
	method.Call(argsVal)
}

func CallMethodWithValue[R any](methodName string, caller any, args ...any) R {
	method := reflect.ValueOf(caller).MethodByName(methodName)
	argsVal := lo.Map(args, func(arg any, _ int) reflect.Value {
		return reflect.ValueOf(arg)
	})
	res := method.Call(argsVal)
	return res[0].Interface().(R)
}

func CallMethodWithError[R any](methodName string, caller any, args ...any) (R, error) {
	method := reflect.ValueOf(caller).MethodByName(methodName)
	argsVal := lo.Map(args, func(arg any, _ int) reflect.Value {
		return reflect.ValueOf(arg)
	})
	res := method.Call(argsVal)
	if res[1].IsNil() {
		return res[0].Interface().(R), nil
	}
	return lo.Empty[R](), res[1].Interface().(error)
}
