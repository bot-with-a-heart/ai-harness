package lmstudio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/providers"
)

const defaultTimeout = 10 * time.Minute

type Client struct {
	name       string
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

func New(name string, cfg appconfig.Provider, opts ...Option) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("lmstudio base_url is required")
	}
	if name == "" {
		name = "lmstudio"
	}

	client := &Client{
		name:    name,
		baseURL: strings.TrimSpace(cfg.BaseURL),
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(client)
	}
	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: defaultTimeout}
	}

	if _, err := client.endpoint("models"); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Health(ctx context.Context) error {
	_, err := c.ListModels(ctx)
	return err
}

func (c *Client) ListModels(ctx context.Context) ([]providers.Model, error) {
	var response modelsResponse
	if err := c.doJSON(ctx, http.MethodGet, "models", nil, &response); err != nil {
		return nil, err
	}

	models := make([]providers.Model, 0, len(response.Data))
	for _, model := range response.Data {
		if model.ID == "" {
			continue
		}
		models = append(models, providers.Model{
			ID:      model.ID,
			Object:  model.Object,
			Created: model.Created,
			OwnedBy: model.OwnedBy,
		})
	}

	return models, nil
}

func (c *Client) Ask(ctx context.Context, req providers.AskRequest) (providers.AskResponse, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return providers.AskResponse{}, errors.New("prompt is required")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		models, err := c.ListModels(ctx)
		if err != nil {
			return providers.AskResponse{}, fmt.Errorf("discover default model: %w", err)
		}
		if len(models) == 0 {
			return providers.AskResponse{}, errors.New("discover default model: LM Studio returned no models")
		}
		model = models[0].ID
	}

	payload := chatCompletionRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	var response chatCompletionResponse
	if err := c.doJSON(ctx, http.MethodPost, "chat/completions", payload, &response); err != nil {
		return providers.AskResponse{}, err
	}
	if len(response.Choices) == 0 {
		return providers.AskResponse{}, errors.New("chat completion returned no choices")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		return providers.AskResponse{}, errors.New("chat completion returned an empty response")
	}
	if response.Model != "" {
		model = response.Model
	}

	return providers.AskResponse{
		Model:   model,
		Content: content,
	}, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, target any) error {
	endpoint, err := c.endpoint(path)
	if err != nil {
		return err
	}

	var body io.Reader
	if payload != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = &buf
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("%s %s returned %s: %s", method, endpoint, resp.Status, strings.TrimSpace(string(preview)))
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response from %s: %w", endpoint, err)
	}

	return nil
}

func (c *Client) endpoint(path string) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse LM Studio base_url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("LM Studio base_url must include scheme and host: %s", c.baseURL)
	}

	basePath := strings.TrimRight(u.Path, "/")
	if basePath == "" {
		basePath = "/v1"
	}

	u.Path = basePath + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = ""
	u.Fragment = ""

	return u.String(), nil
}

type modelsResponse struct {
	Data []modelResponse `json:"data"`
}

type modelResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}
