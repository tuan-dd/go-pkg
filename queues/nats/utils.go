package natsQueue

import (
	"github.com/tuan-dd/go-common/compress"
	"github.com/tuan-dd/go-common/queue"
	"github.com/tuan-dd/go-common/response"
)

func EnBodyCompression(msg *queue.Message) *response.AppError {
	if msg.Header == nil {
		return nil
	}

	ct, oke := (*msg.Header)[string(Compression)]
	if !oke {
		return nil
	}
	var ctStr string
	switch v := ct.(type) {
	case string:
		ctStr = v
	case compress.CompressionType:
		ctStr = string(v)
	}

	var err *response.AppError

	msg.Body, err = compress.Encode(msg.Body, ctStr)
	if err != nil {
		return err
	}
	return nil
}

func DeBodyCompression(msg *queue.Message) *response.AppError {
	if msg.Header == nil {
		return nil
	}

	ct, oke := (*msg.Header)[string(Compression)]
	if !oke {
		return nil
	}
	var ctStr string
	switch v := ct.(type) {
	case string:
		ctStr = v
	case compress.CompressionType:
		ctStr = string(v)
	}

	var err *response.AppError

	msg.Body, err = compress.Decode(msg.Body, ctStr)
	if err != nil {
		return err
	}
	return nil
}
