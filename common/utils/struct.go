package utils

import "encoding/json"

func StructToStruct[T, U any](t *T) (*U, error) {
	var u U
	b, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &u)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func MapToStruct[T any](data any, out *T) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, out)
}

func BytesToStruct[T any](data []byte) (*T, error) {
	var out *T
	return out, json.Unmarshal(data, out)
}
