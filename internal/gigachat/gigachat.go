package gigachat

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	envAuthKey  = "GIGACHAT_AUTH_KEY"
	envScope    = "GIGACHAT_SCOPE"
	envModel    = "GIGACHAT_MODEL"
	envOAuthURL = "GIGACHAT_OAUTH_URL"
	envChatURL  = "GIGACHAT_CHAT_URL"
	envInsecure = "GIGACHAT_INSECURE_TLS"

	defaultOAuthURL = "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"
	defaultChatURL  = "https://gigachat.devices.sberbank.ru/api/v1/chat/completions"
	defaultModel    = "GigaChat-Max"
	defaultScope    = "GIGACHAT_API_PERS"

	headerAuthorization = "Authorization"
	headerContentType   = "Content-Type"
	headerAccept        = "Accept"
	headerRqUID         = "RqUID"
	mimeJSON            = "application/json"
	mimeForm            = "application/x-www-form-urlencoded"
	schemeBearer        = "Bearer "
	schemeBasic         = "Basic "

	roleSystem = "system"
	roleUser   = "user"

	httpTimeout   = 120 * time.Second
	tokenSkew     = 60 * time.Second
	tokenFallback = 25 * time.Minute
	temperature   = 0.1
)

type Client struct {
	http     *http.Client
	authKey  string
	scope    string
	model    string
	oauthURL string
	chatURL  string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func NewFromEnv() *Client {
	tr := &http.Transport{}
	if envBool(envInsecure) {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		http:     &http.Client{Timeout: httpTimeout, Transport: tr},
		authKey:  strings.TrimSpace(os.Getenv(envAuthKey)),
		scope:    envOr(envScope, defaultScope),
		model:    envOr(envModel, defaultModel),
		oauthURL: envOr(envOAuthURL, defaultOAuthURL),
		chatURL:  envOr(envChatURL, defaultChatURL),
	}
}

func (c *Client) Model() string { return c.model }

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.tokenExp) > tokenSkew {
		return c.token, nil
	}
	if c.authKey == "" {
		return "", fmt.Errorf("set %s", envAuthKey)
	}
	form := url.Values{"scope": {c.scope}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.oauthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set(headerAuthorization, schemeBasic+c.authKey)
	req.Header.Set(headerRqUID, newUUID())
	req.Header.Set(headerContentType, mimeForm)
	req.Header.Set(headerAccept, mimeJSON)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gigachat oauth: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("gigachat oauth http %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("gigachat oauth decode: %w (raw: %s)", err, truncate(string(raw), 200))
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("gigachat oauth: empty access_token (raw: %s)", truncate(string(raw), 200))
	}
	c.token = out.AccessToken
	if out.ExpiresAt > 0 {
		c.tokenExp = time.UnixMilli(out.ExpiresAt)
	} else {
		c.tokenExp = time.Now().Add(tokenFallback)
	}
	return c.token, nil
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float32   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) ChatJSON(ctx context.Context, system, user string) (string, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []message{
			{Role: roleSystem, Content: system},
			{Role: roleUser, Content: user},
		},
		Temperature: temperature,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set(headerAuthorization, schemeBearer+token)
	req.Header.Set(headerContentType, mimeJSON)
	req.Header.Set(headerAccept, mimeJSON)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("gigachat chat http %d: %s", resp.StatusCode, string(raw))
	}
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("gigachat chat decode: %w (raw: %s)", err, truncate(string(raw), 300))
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("gigachat: empty choices (raw: %s)", truncate(string(raw), 300))
	}
	return out.Choices[0].Message.Content, nil
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
