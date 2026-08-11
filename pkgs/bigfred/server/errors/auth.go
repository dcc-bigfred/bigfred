package errors

import "errors"

const (
	CodeInvalidCredentials = "invalid_credentials"
	CodeAccountDeactivated = "account_deactivated"

	CodeOAuthInvalidClient       = "invalid_client"
	CodeOAuthInvalidRedirectURI  = "invalid_redirect_uri"
	CodeOAuthInvalidGrant        = "invalid_grant"
	CodeOAuthExpiredCode         = "expired_code"
	CodeOAuthInvalidClientSecret = "invalid_client_secret"
	CodeImpersonationForbidden   = "impersonation_forbidden"
	CodeDCCPoolExhausted         = "dcc_pool_exhausted"
)

var (
	// ErrInvalidCredentials intentionally covers unknown login and wrong PIN.
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDeactivated = errors.New(CodeAccountDeactivated)

	ErrOAuthInvalidClient       = errors.New(CodeOAuthInvalidClient)
	ErrOAuthInvalidRedirectURI  = errors.New(CodeOAuthInvalidRedirectURI)
	ErrOAuthInvalidGrant        = errors.New(CodeOAuthInvalidGrant)
	ErrOAuthExpiredCode         = errors.New(CodeOAuthExpiredCode)
	ErrOAuthInvalidClientSecret = errors.New(CodeOAuthInvalidClientSecret)
	ErrImpersonationForbidden   = errors.New(CodeImpersonationForbidden)
	ErrDCCPoolExhausted         = errors.New(CodeDCCPoolExhausted)
)
