package gRPC

import (
	"github.com/tuan-dd/go-common/response"
	"gitlab.betmaker365.com/dev-viet/smm-document/api/common"
)

func TransformError(err *response.AppError) *common.ErrMsg {
	if err == nil {
		return nil
	}
	return &common.ErrMsg{
		Message: err.Message,
	}
}
