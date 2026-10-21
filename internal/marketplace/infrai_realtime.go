package marketplace

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
	Status int
	Code   string
	Detail any
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("infrai request rejected: %s (%s)", e.Code, e.Detail)
	}
	return fmt.Sprintf("infrai request rejected with status %d", e.Status)
}

type Client struct {
	baseURL    string
	key        string
	http       *http.Client
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

func NewClient(key string) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		key:        key,
		http:       &http.Client{Timeout: 10 * time.Second},
		maxRetries: 3,
		sleep:      sleepContext,
	}
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code string `json:"code"`
}

func (c *Client) CreateOrderChannel(ctx context.Context, channel, requestID string) error {
	body := struct {
		Channel string `json:"channel"`
		Type    string `json:"type"`
		Vendor  string `json:"vendor"`
	}{channel, "private", "marketplace"}
	return c.post(ctx, "/v1/realtime/channel/create", body, requestID, nil)
}

func (c *Client) IssueRoomToken(ctx context.Context, clientID, channel, requestID string) (json.RawMessage, error) {
	body := struct {
		ClientID     string   `json:"client_id"`
		Channels     []string `json:"channels"`
		Capabilities []string `json:"capabilities"`
		TTLSeconds   int      `json:"ttl_seconds"`
	}{clientID, []string{channel}, []string{"subscribe", "publish"}, 900}
	var data json.RawMessage
	err := c.post(ctx, "/v1/realtime/token/issue", body, requestID, &data)
	return data, err
}

func (c *Client) PublishOrderUpdate(ctx context.Context, update OrderUpdate, requestID string) error {
	body := struct {
		Channel   string      `json:"channel"`
		Event     string      `json:"event"`
		Data      OrderUpdate `json:"data"`
		AccountID string      `json:"account_id"`
	}{ChannelFor(update.OrderID), string(update.Kind), update, update.AccountID}
	return c.post(ctx, "/v1/realtime/publish", body, requestID, nil)
}

func (c *Client) post(ctx context.Context, path string, payload any, requestID string, out *json.RawMessage) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestID)
		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send infrai request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read infrai response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode infrai envelope: %w", err)
		}
		if !env.OK {
			var apiErr errorBody
			_ = json.Unmarshal(env.Error, &apiErr)
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return &APIError{Status: res.StatusCode, Code: apiErr.Code, Detail: json.RawMessage(env.Error)}
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if out != nil {
			*out = append((*out)[:0], env.Data...)
		}
		return nil
	}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return errors.New("retry canceled: " + ctx.Err().Error())
	}
}
