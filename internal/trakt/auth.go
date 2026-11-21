package trakt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cecobask/imdb-trakt-sync/internal/config"
)

// An initial PKCE authorization happens once on the user's Mac using
// scripts/trakt_pkce_login.py. Only already-authorized tokens are used here.
// No Trakt Client Secret is required.
type authClient struct {
	baseURL string
	client  *http.Client
	conf    config.Trakt
}

type authTokensBody struct {
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
	RedirectURI  string `json:"redirect_uri"`
	GrantType    string `json:"grant_type"`
}

type authTokensResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresIn    int64  `json:"expires_in"`
}

type authTransport struct {
	authClient    *authClient
	next          http.RoundTripper
	accessToken   *accessToken
	clientID      string
	tokenFilePath string
	logger        *slog.Logger
}

type accessToken struct {
	token        string
	refreshToken string
	expiresAt    time.Time
}

func newAuthClient(conf config.Trakt, transport http.RoundTripper) *authClient {
	return &authClient{
		baseURL: pathBaseAPI,
		client:  &http.Client{Transport: transport, Timeout: 25 * time.Second},
		conf:    conf,
	}
}

func newAuthTransport(next http.RoundTripper, ac *authClient, clientID, tokenFilePath string, logger *slog.Logger) *authTransport {
	return &authTransport{
		authClient:    ac,
		next:          next,
		clientID:      clientID,
		tokenFilePath: tokenFilePath,
		logger:        logger,
	}
}

func (at *accessToken) isExpired() bool {
	// Refresh shortly before expiry to avoid starting a long sync with a token
	// that expires while requests are underway.
	return at != nil && !time.Now().Add(3*time.Minute).Before(at.expiresAt)
}

func (ac *authClient) refreshAccessToken(ctx context.Context, refreshToken string) (*accessToken, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("missing Trakt refresh token; run the local PKCE login again")
	}
	payload := authTokensBody{
		ClientID:     *ac.conf.ClientID,
		RefreshToken: refreshToken,
		RedirectURI:  *ac.conf.RedirectURI,
		GrantType:    "refresh_token",
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling Trakt refresh request: %w", err)
	}
	resp, err := doRequest(ctx, ac.client, http.MethodPost, ac.baseURL, "/oauth/token", nil, bytes.NewReader(b), nil, http.StatusOK)
	if err != nil {
		return nil, fmt.Errorf("refreshing Trakt token (re-authorize with the local PKCE script if invalid_grant): %w", err)
	}
	data, err := decodeJSON[*authTokensResponse](resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decoding Trakt refresh response: %w", err)
	}
	if data == nil || data.AccessToken == "" || data.RefreshToken == "" || data.ExpiresIn <= 0 {
		return nil, fmt.Errorf("Trakt refresh response did not include a valid token pair")
	}
	createdAt := data.CreatedAt
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	return &accessToken{token: data.AccessToken, refreshToken: data.RefreshToken, expiresAt: time.Unix(createdAt+data.ExpiresIn, 0)}, nil
}

func (at *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	if at.accessToken == nil {
		stored, err := loadToken(at.tokenFilePath)
		if err != nil {
			return nil, fmt.Errorf("loading Trakt token: %w", err)
		}
		if stored == nil || stored.AccessToken == "" || stored.RefreshToken == "" {
			return nil, fmt.Errorf("missing Trakt token file: authorize once on the Mac with scripts/trakt_pkce_login.py and save the JSON as the TRAKT_TOKEN GitHub secret")
		}
		at.accessToken = stored.toAccessToken()
	}
	if at.accessToken.isExpired() {
		// Trakt refresh tokens are single-use. Refresh once, persist the newly
		// issued pair, then use it. The GitHub workflow must save it back.
		fresh, err := at.authClient.refreshAccessToken(ctx, at.accessToken.refreshToken)
		if err != nil {
			return nil, err
		}
		if err := saveToken(at.tokenFilePath, &storedToken{
			AccessToken:  fresh.token,
			RefreshToken: fresh.refreshToken,
			ExpiresAt:    fresh.expiresAt,
		}); err != nil {
			return nil, fmt.Errorf("saving newly rotated Trakt tokens (manual re-authentication may be needed): %w", err)
		}
		at.accessToken = fresh
		at.logger.Info("refreshed Trakt access token")
	}
	req.Header.Set("authorization", "Bearer "+at.accessToken.token)
	req.Header.Set("trakt-api-key", at.clientID)
	req.Header.Set("trakt-api-version", "2")
	return at.next.RoundTrip(req)
}
