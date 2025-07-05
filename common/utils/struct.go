package utils

import (
	"github.com/bytedance/sonic"
)

func StructToStruct[T, U any](t *T) (*U, error) {
	var u U
	b, err := sonic.Marshal(t)
	if err != nil {
		return nil, err
	}

	err = sonic.Unmarshal(b, &u)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func MapToStruct[T any](data any, out *T) error {
	bytes, err := sonic.Marshal(data)
	if err != nil {
		return err
	}
	return sonic.Unmarshal(bytes, out)
}
