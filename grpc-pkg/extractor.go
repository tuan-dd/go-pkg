package gRPC

import (
	"context"
	"strconv"
	"strings"

	"github.com/tuan-dd/go-common/constants"
	"google.golang.org/grpc/metadata"
)

type (
	Extractor interface {
		Get(ctx context.Context, name string) []string
		GetFirst(ctx context.Context, name string) string
		GetTokenID(ctx context.Context) string
		GetTenantID(ctx context.Context) string
		GetUserID(ctx context.Context) (string, bool)
		GetUsername(ctx context.Context) string
		GetGroupIDs(ctx context.Context) []string
		GetXForwardedFor(ctx context.Context) string
		GetUtmSource(ctx context.Context) string
		GetLabelIDs(ctx context.Context) []string
		GetLastTenSignInDate(ctx context.Context) []string
		GetAppID(ctx context.Context) string
		GetRequestID(ctx context.Context) string
		GetRequestTimestamp(ctx context.Context) int64
		GetLanguageCode(ctx context.Context) string
		GetRole(ctx context.Context) string
	}

	extractor struct{}
)

func NewExtractor() Extractor {
	return &extractor{}
}

func (t *extractor) Get(ctx context.Context, name string) []string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}

	return md.Get(name)
}

func (t *extractor) GetFirst(ctx context.Context, name string) string {
	values := t.Get(ctx, name)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (t *extractor) GetTokenID(ctx context.Context) string {
	values := t.Get(ctx, string(constants.TokenID))
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (t *extractor) GetTenantID(ctx context.Context) string {
	values := t.Get(ctx, string(constants.TenantID))
	if len(values) == 0 {
		// Return empty in case TenantID is undefined
		// Empty is default TenantID
		return ""
	}

	return values[0]
}

func (t *extractor) GetUserID(ctx context.Context) (string, bool) {
	values := t.Get(ctx, string(constants.UserID))
	if len(values) == 0 {
		return "", false
	}

	return values[0], true
}

func (t *extractor) GetUsername(ctx context.Context) string {
	values := t.Get(ctx, string(constants.UserName))
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (t *extractor) GetGroupIDs(ctx context.Context) []string {
	return t.Get(ctx, string(constants.GroupID))
}

func (t *extractor) GetXForwardedFor(ctx context.Context) string {
	values := t.Get(ctx, string(constants.XForwardedFor))
	if len(values) == 0 {
		return ""
	}

	return strings.Join(values[:], ",")
}

func (t *extractor) GetUtmSource(ctx context.Context) string {
	values := t.Get(ctx, string(constants.XUtmSource))
	if len(values) == 0 {
		return ""
	}

	return strings.Join(values[:], ",")
}

func (t *extractor) GetLabelIDs(ctx context.Context) []string {
	return t.Get(ctx, string(constants.XLabelIDs))
}

func (t *extractor) GetLastTenSignInDate(ctx context.Context) []string {
	return t.Get(ctx, string(constants.XLastTenSignInDate))
}

func (t *extractor) GetAppID(ctx context.Context) string {
	return t.GetFirst(ctx, string(constants.XAppID))
}

func (t *extractor) GetRequestID(ctx context.Context) string {
	return t.GetFirst(ctx, string(constants.REQUEST_ID_KEY))
}

func (t *extractor) GetRequestTimestamp(ctx context.Context) int64 {
	s := t.GetFirst(ctx, string(constants.StartTime))

	timestamp, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return int64(timestamp * 1000)
	}

	return 0
}

func (t *extractor) GetRequestPath(ctx context.Context) string {
	return t.GetFirst(ctx, string(constants.Path))
}

func (t *extractor) GetLanguageCode(ctx context.Context) string {
	return t.GetFirst(ctx, string(constants.LANGUAGE_CODE_HEADER_KEY))
}

func (t *extractor) GetRole(ctx context.Context) string {
	values := t.Get(ctx, string(constants.RoleID))
	if len(values) == 0 {
		return ""
	}

	return values[0]
}
