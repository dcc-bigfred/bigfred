package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

// OAuthHandler serves /api/v1/auth/oauth/*.
type OAuthHandler struct {
	oauth *cmd.OAuth
	auth  *cmd.Auth
}

// NewOAuthHandler constructs the OAuth HTTP handlers.
func NewOAuthHandler(oauth *cmd.OAuth, auth *cmd.Auth) *OAuthHandler {
	return &OAuthHandler{oauth: oauth, auth: auth}
}

// Authorize handles GET /api/v1/auth/oauth/authorize (browser redirect).
// Optional `layout_id` pins the issued code (and thus the wizard JWT) to
// that makieta — used by bigfred-wizard's pre-SSO layout picker.
func (h *OAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := strings.TrimSpace(q.Get("client_id"))
	redirectURI := strings.TrimSpace(q.Get("redirect_uri"))
	state := q.Get("state")
	responseType := strings.TrimSpace(q.Get("response_type"))
	preferredLayoutID := parseOptionalUint(q.Get("layout_id"))
	if responseType != "" && responseType != "code" {
		writeJSONError(w, http.StatusBadRequest, "unsupported_response_type")
		return
	}
	if _, err := h.oauth.AuthorizeValidatedClient(clientID, redirectURI); err != nil {
		status, code := oauthHTTPStatus(err)
		writeJSONErrorCause(w, status, code, err)
		return
	}

	token := readSessionToken(r)
	if token == "" {
		h.redirectToLogin(w, r, preferredLayoutID)
		return
	}
	id, err := h.auth.VerifyToken(r.Context(), token)
	if err != nil {
		h.redirectToLogin(w, r, preferredLayoutID)
		return
	}

	if preferredLayoutID != 0 && preferredLayoutID != id.Layout.ID {
		bound, err := h.auth.IdentityForLayout(r.Context(), id.User, preferredLayoutID)
		if err != nil {
			status, code := svcerrors.LayoutHTTPStatus(err)
			writeJSONErrorCause(w, status, code, err)
			return
		}
		id = bound
	}

	code, err := h.oauth.IssueCode(r.Context(), id, clientID, redirectURI)
	if err != nil {
		status, code := oauthHTTPStatus(err)
		writeJSONErrorCause(w, status, code, err)
		return
	}
	u, err := url.Parse(redirectURI)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, svcerrors.CodeOAuthInvalidRedirectURI)
		return
	}
	qq := u.Query()
	qq.Set("code", code)
	if state != "" {
		qq.Set("state", state)
	}
	u.RawQuery = qq.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (h *OAuthHandler) redirectToLogin(w http.ResponseWriter, r *http.Request, layoutID uint) {
	returnTo := r.URL.RequestURI()
	if !strings.HasPrefix(returnTo, "/api/v1/auth/oauth/authorize") {
		returnTo = "/api/v1/auth/oauth/authorize?" + r.URL.RawQuery
	}
	params := url.Values{}
	params.Set("return_to", returnTo)
	if layoutID != 0 {
		params.Set("layout_id", strconv.FormatUint(uint64(layoutID), 10))
	}
	http.Redirect(w, r, "/login?"+params.Encode(), http.StatusFound)
}

type oauthTokenRequest struct {
	GrantType    string `json:"grantType"`
	Code         string `json:"code"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURI  string `json:"redirectUri"`
}

type oauthTokenResponse struct {
	AccessToken string    `json:"accessToken"`
	TokenType   string    `json:"tokenType"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// Token handles POST /api/v1/auth/oauth/token.
func (h *OAuthHandler) Token(w http.ResponseWriter, r *http.Request) {
	var body oauthTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Also accept form-urlencoded for compatibility.
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "bad_request")
			return
		}
		body = oauthTokenRequest{
			GrantType:    firstNonEmpty(r.Form.Get("grant_type"), r.Form.Get("grantType")),
			Code:         r.Form.Get("code"),
			ClientID:     firstNonEmpty(r.Form.Get("client_id"), r.Form.Get("clientId")),
			ClientSecret: firstNonEmpty(r.Form.Get("client_secret"), r.Form.Get("clientSecret")),
			RedirectURI:  firstNonEmpty(r.Form.Get("redirect_uri"), r.Form.Get("redirectUri")),
		}
	} else if body.GrantType == "" {
		body.GrantType = "authorization_code"
	}
	res, err := h.oauth.ExchangeCode(r.Context(), cmd.TokenExchangeInput{
		GrantType:    body.GrantType,
		Code:         body.Code,
		ClientID:     body.ClientID,
		ClientSecret: body.ClientSecret,
		RedirectURI:  body.RedirectURI,
	})
	if err != nil {
		status, code := oauthHTTPStatus(err)
		writeJSONErrorCause(w, status, code, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(oauthTokenResponse{
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   res.ExpiresAt,
	})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseOptionalUint(raw string) uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		return 0
	}
	return uint(n)
}

func oauthHTTPStatus(err error) (int, string) {
	switch {
	case errors.Is(err, svcerrors.ErrOAuthInvalidClient):
		return http.StatusUnauthorized, svcerrors.CodeOAuthInvalidClient
	case errors.Is(err, svcerrors.ErrOAuthInvalidRedirectURI):
		return http.StatusBadRequest, svcerrors.CodeOAuthInvalidRedirectURI
	case errors.Is(err, svcerrors.ErrOAuthInvalidGrant):
		return http.StatusBadRequest, svcerrors.CodeOAuthInvalidGrant
	case errors.Is(err, svcerrors.ErrOAuthExpiredCode):
		return http.StatusBadRequest, svcerrors.CodeOAuthExpiredCode
	case errors.Is(err, svcerrors.ErrOAuthInvalidClientSecret):
		return http.StatusUnauthorized, svcerrors.CodeOAuthInvalidClientSecret
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}
