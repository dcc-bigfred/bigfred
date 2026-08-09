package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const oauthCodeTTL = 60 * time.Second

// OAuth handles authorization-code issue and token exchange.
type OAuth struct {
	auth    *Auth
	clients *OAuthClientsRegistry
	redis   *redis.Client
}

// NewOAuth wires OAuth2 code flow helpers.
func NewOAuth(auth *Auth, clients *OAuthClientsRegistry, rdb *redis.Client) *OAuth {
	return &OAuth{auth: auth, clients: clients, redis: rdb}
}

type oauthCodePayload struct {
	UserID       uint   `json:"uid"`
	LayoutID     uint   `json:"lid"`
	ClientID     string `json:"clientId"`
	RedirectURI  string `json:"redirectUri"`
}

// AuthorizeValidatedClient returns the client or ErrOAuthInvalidClient /
// redirect errors.
func (o *OAuth) AuthorizeValidatedClient(clientID, redirectURI string) (OAuthClient, error) {
	c, ok := o.clients.Get(clientID)
	if !ok {
		return OAuthClient{}, svcerrors.ErrOAuthInvalidClient
	}
	if !c.RedirectURIAllowed(redirectURI) {
		return OAuthClient{}, svcerrors.ErrOAuthInvalidRedirectURI
	}
	return c, nil
}

// IssueCode stores a one-time code for the authenticated identity.
func (o *OAuth) IssueCode(ctx context.Context, id Identity, clientID, redirectURI string) (string, error) {
	if _, err := o.AuthorizeValidatedClient(clientID, redirectURI); err != nil {
		return "", err
	}
	if o.redis == nil {
		return "", fmt.Errorf("oauth: redis unavailable")
	}
	code, err := randomHex(16)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(oauthCodePayload{
		UserID:      id.User.ID,
		LayoutID:    id.Layout.ID,
		ClientID:    clientID,
		RedirectURI: redirectURI,
	})
	if err != nil {
		return "", err
	}
	key := oauthCodeKey(code)
	if err := o.redis.Set(ctx, key, payload, oauthCodeTTL).Err(); err != nil {
		return "", err
	}
	return code, nil
}

// TokenExchangeInput is the body of POST /auth/oauth/token.
type TokenExchangeInput struct {
	GrantType    string
	Code         string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// TokenExchangeResult is returned to the OAuth client.
type TokenExchangeResult struct {
	AccessToken string
	TokenType   string
	ExpiresAt   time.Time
}

// ExchangeCode validates client credentials and returns a session JWT.
func (o *OAuth) ExchangeCode(ctx context.Context, in TokenExchangeInput) (TokenExchangeResult, error) {
	if strings.TrimSpace(in.GrantType) != "authorization_code" {
		return TokenExchangeResult{}, svcerrors.ErrOAuthInvalidGrant
	}
	c, err := o.AuthorizeValidatedClient(in.ClientID, in.RedirectURI)
	if err != nil {
		return TokenExchangeResult{}, err
	}
	if c.ClientSecret == "" || c.ClientSecret != in.ClientSecret {
		return TokenExchangeResult{}, svcerrors.ErrOAuthInvalidClientSecret
	}
	if o.redis == nil {
		return TokenExchangeResult{}, fmt.Errorf("oauth: redis unavailable")
	}
	key := oauthCodeKey(in.Code)
	raw, err := o.redis.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return TokenExchangeResult{}, svcerrors.ErrOAuthInvalidGrant
		}
		return TokenExchangeResult{}, err
	}
	var p oauthCodePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return TokenExchangeResult{}, svcerrors.ErrOAuthInvalidGrant
	}
	if p.ClientID != in.ClientID || p.RedirectURI != in.RedirectURI {
		return TokenExchangeResult{}, svcerrors.ErrOAuthInvalidGrant
	}
	user, err := o.auth.users.FindByID(ctx, p.UserID)
	if err != nil {
		return TokenExchangeResult{}, svcerrors.ErrOAuthInvalidGrant
	}
	if !user.Active {
		return TokenExchangeResult{}, svcerrors.ErrAccountDeactivated
	}
	layout, err := o.auth.layouts.ValidateForLogin(ctx, p.LayoutID)
	if err != nil {
		return TokenExchangeResult{}, err
	}
	token, exp, err := o.auth.IssueToken(Identity{User: user, Layout: layout})
	if err != nil {
		return TokenExchangeResult{}, err
	}
	return TokenExchangeResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   exp,
	}, nil
}

func oauthCodeKey(code string) string {
	return "oauth:code:" + code
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
