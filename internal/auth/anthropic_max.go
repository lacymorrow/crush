package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const anthropicClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

type pkce struct {
	Verifier  string
	Challenge string
}

func generatePKCE() (pkce, error) {
	// 43-128 characters recommended length
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		return pkce{}, err
	}
	// base64url no padding
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return pkce{Verifier: verifier, Challenge: challenge}, nil
}

// AuthorizeURL builds the Claude Max authorization URL and returns state/verifier for PKCE.
// mode: "max" or "console" (same as opencode)
func AuthorizeURL(mode string) (authorizeURL string, verifier string, err error) {
	pk, err := generatePKCE()
	if err != nil {
		return "", "", err
	}
	domain := "claude.ai"
	if mode == "console" {
		domain = "console.anthropic.com"
	}
	u := &url.URL{
		Scheme: "https",
		Host:   domain,
		Path:   "/oauth/authorize",
	}
	q := u.Query()
	q.Set("code", "true")
	q.Set("client_id", anthropicClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", "https://console.anthropic.com/oauth/code/callback")
	q.Set("scope", "org:create_api_key user:profile user:inference")
	q.Set("code_challenge", pk.Challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", pk.Verifier)
	u.RawQuery = q.Encode()
	return u.String(), pk.Verifier, nil
}

type tokenResponse struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// ExchangeCode exchanges the pasted code#state for tokens.
func ExchangeCode(codeWithState string, verifier string) (OauthInfo, error) {
	parts := strings.Split(codeWithState, "#")
	if len(parts) < 2 {
		return OauthInfo{}, errors.New("invalid code format")
	}
	body := map[string]any{
		"code":          parts[0],
		"state":         parts[1],
		"grant_type":    "authorization_code",
		"client_id":     anthropicClientID,
		"redirect_uri":  "https://console.anthropic.com/oauth/code/callback",
		"code_verifier": verifier,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://console.anthropic.com/v1/oauth/token", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return OauthInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OauthInfo{}, fmt.Errorf("exchange failed: %s", resp.Status)
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return OauthInfo{}, err
	}
	return OauthInfo{
		Type:    "oauth",
		Refresh: tr.RefreshToken,
		Access:  tr.AccessToken,
		Expires: time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second).UnixMilli(),
	}, nil
}

// AccessToken returns a fresh access token for the anthropic provider, refreshing if needed.
func AccessToken(providerID string) (string, error) {
	info, err := Get(providerID)
	if err != nil || info == nil || info.Type != "oauth" {
		return "", err
	}
	if info.Access != "" && info.Expires > time.Now().UnixMilli() {
		return info.Access, nil
	}
	// refresh
	body := map[string]any{
		"grant_type":    "refresh_token",
		"refresh_token": info.Refresh,
		"client_id":     anthropicClientID,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://console.anthropic.com/v1/oauth/token", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("refresh failed: %s", resp.Status)
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	updated := OauthInfo{
		Type:    "oauth",
		Refresh: tr.RefreshToken,
		Access:  tr.AccessToken,
		Expires: time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second).UnixMilli(),
	}
	if err := Set(providerID, updated); err != nil {
		return "", err
	}
	return updated.Access, nil
}
