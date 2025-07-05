package utils

import (
	"os"

	"github.com/bytedance/sonic"
)

func IsEmpty[T any](arr []T) bool {
	if arr == nil {
		return true
	}

	return len(arr) == 0
}

func IsBlank(v *string) bool {
	if v == nil {
		return true
	}

	return len(*v) == 0
}

func WriteFile(name string, data any) {
	b, _ := sonic.Marshal(data)
	_ = os.WriteFile(name, b, 0o644)
}
