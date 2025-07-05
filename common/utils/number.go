package utils

import (
	"strconv"
)

func ToInt[T int | int8 | int16 | int32 | int64](s string) T {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}

	return T(val)
}

func ToIntPtrFromInt[T int | int8 | int16 | int32 | int64, V int | int8 | int16 | int32 | int64](ptr *V) *T {
	if ptr == nil {
		return nil
	}

	new := T(*ptr)
	return &new
}

func ToFloatPtrFromFloat[T float32 | float64, V float32 | float64](ptr *V) *T {
	if ptr == nil {
		return nil
	}

	new := T(*ptr)
	return &new
}
