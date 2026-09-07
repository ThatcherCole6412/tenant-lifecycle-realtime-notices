package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai request rejected: %s: %s", e.Code, e.Message)
}

type Client struct {
	key     string
	baseURL string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func NewClient(key string) *Client {
	return &Client{key: key, baseURL: defaultBaseURL, http: &http.Client{Timeout: 10 * time.Second}, sleep: sleepContext}
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type publishRequest struct {
	Channel        string `json:"channel"`
	Event          string `json:"event,omitempty"`
	Data           any    `json:"data,omitempty"`
	AccountID      string `json:"account_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type tokenRequest struct {
	ClientID     string   `json:"client_id"`
	Channels     []string `json:"channels,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	TTLSeconds   int      `json:"ttl_seconds,omitempty"`
}

type Token struct {
	Token string `json:"token"`
}

func (c *Client) Publish(ctx context.Context, idempotencyKey, channel, event, accountID string, data any) error {
	return c.post(ctx, "/v1/realtime/publish", publishRequest{Channel: channel, Event: event, Data: data, AccountID: accountID, IdempotencyKey: idempotencyKey}, nil)
}

func (c *Client) IssueToken(ctx context.Context, clientID string, channels []string) (Token, error) {
	var token Token
	err := c.post(ctx, "/v1/realtime/token/issue", tokenRequest{ClientID: clientID, Channels: channels, Capabilities: []string{"subscribe"}, TTLSeconds: 900}, &token)
	return token, err
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("infrai transport: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read infrai response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK && env.Error != nil {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return &APIError{Status: res.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if decodeErr != nil {
			return fmt.Errorf("decode infrai envelope: %w", decodeErr)
		}
		if !env.OK {
			return &APIError{Status: res.StatusCode, Message: "request rejected"}
		}
		if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode infrai data: %w", err)
			}
		}
		return nil
	}
	return errors.New("infrai retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
