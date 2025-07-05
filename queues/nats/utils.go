package natsQueue

import (
	"bytes"
	"fmt"
	"io"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/s2"
	"github.com/tuan-dd/go-pkg/common/queue"
	"github.com/tuan-dd/go-pkg/common/response"
)

func EnBodyCompression(msg *queue.Message) *response.AppError {
	if msg.Headers == nil {
		return nil
	}

	compressionType := NoCompression
	if val, ok := (*msg.Headers)[string(CompressionType(Compression))].(string); ok {
		compressionType = CompressionType(val)
	}

	switch compressionType {
	case S2Compression:
		msg.Body = s2.EncodeBest(nil, msg.Body)
	case GzipCompression:
		buf := getBuffer()
		defer putBuffer(buf)
		gz := getGzipWriter(buf)
		defer putGzipWriter(gz)
		if _, err := gz.Write(msg.Body); err != nil {
			return response.ServerError(fmt.Sprintf("failed to compress message: %s", err))
		}
		if err := gz.Close(); err != nil {
			return response.ServerError(fmt.Sprintf("failed to close gzip writer: %s", err))
		}
		msg.Body = buf.Bytes() // compressed bytes
	case NoCompression:
	default:
		return response.ServerError(fmt.Sprintf("unsupported compression type: %s", compressionType))
	}
	return nil
}

func DeBodyCompression(msg *queue.Message) *response.AppError {
	var err error
	if msg.Headers == nil {
		return nil
	}

	compressionType := NoCompression
	if val, ok := (*msg.Headers)[string(CompressionType(Compression))].(string); ok {
		compressionType = CompressionType(val)
	}

	switch compressionType {
	case S2Compression:
		msg.Body, err = s2.Decode(nil, msg.Body)
		if err != nil {
			return response.ServerError(fmt.Sprintf("failed to decompress s2 message: %s", err))
		}
	case GzipCompression:
		buf := getBuffer()
		defer putBuffer(buf)
		gz, err := gzip.NewReader(bytes.NewReader(msg.Body))
		if err != nil {
			return response.ServerError(fmt.Sprintf("failed to create gzip reader: %s", err))
		}
		defer gz.Close()
		if _, err := io.Copy(buf, gz); err != nil {
			return response.ServerError(fmt.Sprintf("failed to decompress gzip message: %s", err))
		}
		msg.Body = buf.Bytes() // decompressed bytes
	case NoCompression:
	default:
		return response.ServerError(fmt.Sprintf("unsupported compression type: %s", compressionType))
	}
	return nil
}
