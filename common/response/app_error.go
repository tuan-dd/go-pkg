package response

import (
	"fmt"
	"reflect"

	"github.com/tuan-dd/go-pkg/common/constants"
)

type AppError struct {
	Code    constants.InternalCode `json:"code"`
	Data    any                    `json:"data,omitempty"`
	Message string                 `json:"message"`
}

func (r *AppError) IsZero() bool {
	return r.Code == 0
}

func (r *AppError) Error() string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("%d:%s", r.Code, r.Message)
}

func NewAppError(message string, code constants.InternalCode) *AppError {
	return &AppError{Code: code, Message: message}
}

func (r *AppError) WithData(data any) *AppError {
	r.Data = data
	return r
}

func QueryNotFound(message string) *AppError {
	if message == "" {
		message = "query not found"
	}
	return NewAppError(message, constants.QueryNotFound)
}

func QueryInvalid(message string) *AppError {
	if message == "" {
		message = "invalid query"
	}
	return NewAppError(message, constants.ParamInvalid)
}

func Unauthorized(message string) *AppError {
	if message == "" {
		message = "need authenticated for request."
	}
	return NewAppError(message, constants.InvalidToken)
}

func AccessDenied(message string) *AppError {
	if message == "" {
		message = "need authenticated for request."
	}
	return NewAppError(message, constants.AcceptDenied)
}

func UnknownError(message string) *AppError {
	if message == "" {
		message = "server error"
	}
	return NewAppError(message, constants.InternalServerErr)
}

func NotFound(message string) *AppError {
	if message == "" {
		message = "not found"
	}
	return NewAppError(message, constants.NotFound)
}

func Duplicate(message string) *AppError {
	if message == "" {
		message = "duplicate entry"
	}
	return NewAppError(message, constants.DuplicateData)
}

func Conflict(message string) *AppError {
	if message == "" {
		message = "conflict data"
	}
	return NewAppError(message, constants.ConflictData)
}

func ServerError(message string) *AppError {
	if message == "" {
		message = "server error"
	}
	return NewAppError(message, constants.InternalServerErr)
}

func ConvertDatabaseError(err error) *AppError {
	return NewAppError(err.Error(), constants.DatabaseErr)
}

func ConvertError(err error) *AppError {
	if err == nil || reflect.TypeOf(err).Kind() == reflect.Ptr && reflect.ValueOf(err).IsNil() {
		return nil
	}

	if appErr, ok := err.(*AppError); ok {
		return appErr
	}

	return NewAppError(err.Error(), constants.InternalServerErr)
}
