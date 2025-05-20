package common

import (
	"github.com/tuan-dd/go-pkg/common/constants"
	"github.com/tuan-dd/go-pkg/common/response"
)

type ResponseData struct {
	CID     string `json:"cid"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type ErrorResponseData struct {
	CID    string `json:"cid"`
	Code   int    `json:"code"`
	Err    string `json:"error"`
	Detail any    `json:"detail"`
}

// success response
func SuccessResponse(reqCtx *ReqContext, data any) *ResponseData {
	return &ResponseData{
		CID:     reqCtx.CID,
		Code:    int(constants.Success),
		Message: "success",
		Data:    data,
	}
}

func ErrorResponse(reqCtx *ReqContext, err *response.AppError) *ErrorResponseData {
	message := constants.Msg[err.Code]
	if err.Message == "" {
		message = constants.Msg[constants.InternalServerErr]
	}

	return &ErrorResponseData{
		CID:    reqCtx.CID,
		Code:   int(err.Code),
		Err:    message,
		Detail: err.Data,
	}
}
