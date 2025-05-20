package constants

type HeaderKey string

const (
	CORRELATION_ID_KEY       HeaderKey = "correlation-id"
	REQUEST_ID_KEY           HeaderKey = "x-request-id"
	SERVICE_KEY              HeaderKey = "service"
	REQUEST_CONTEXT_KEY      HeaderKey = "x-request-info"
	AUTHORIZATION_KEY        HeaderKey = "Authorization"
	CONTENT_TYPE_HEADER_KEY  HeaderKey = "Content-Type"
	LANGUAGE_CODE_HEADER_KEY HeaderKey = "x-language-code"
	REFRESH_TOKEN_HEADER_KEY HeaderKey = "x-refresh-token"
	TenantID                 HeaderKey = "x-tenant-id"
	TokenID                  HeaderKey = "x-token-id"
	UserID                   HeaderKey = "x-user-id"
	UserName                 HeaderKey = "x-username"
	GroupID                  HeaderKey = "x-group-id"
	RoleID                   HeaderKey = "x-role-id"
	Status                   HeaderKey = "x-user-status"
	XForwardedFor            HeaderKey = "x-forwarded-for"
	XUtmSource               HeaderKey = "x-utm-source"
	XLabelIDs                HeaderKey = "x-label-ids"
	XLastTenSignInDate       HeaderKey = "x-last-ten-sign-in-date"
	XAppID                   HeaderKey = "x-app-id"
	StartTime                HeaderKey = "x-request-timestamp"
	Path                     HeaderKey = "path"
)
