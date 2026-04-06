package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbeddingClient calls the semantic-router embedding service.
type EmbeddingClient struct {
	url     string
	client  *http.Client
	timeout time.Duration
}

// ClassifyRequest is the request payload for /classify endpoint.
type ClassifyRequest struct {
	Text string `json:"text"`
}

// ClassifyResponse is the response from /classify endpoint.
type ClassifyResponse struct {
	RouteName      string  `json:"route_name"`
	Tier           string  `json:"tier"`
	Reasoning      string  `json:"reasoning"`
	TaskType       string  `json:"task_type"`
	Modality       string  `json:"modality"`
	Confidence     float64 `json:"confidence"`
	FallbackReason string  `json:"fallback_reason,omitempty"`
}

// NewEmbeddingClient creates a new embedding service client.
func NewEmbeddingClient(url string, timeout time.Duration) *EmbeddingClient {
	if timeout == 0 {
		timeout = 500 * time.Millisecond // Default 500ms timeout
	}
	return &EmbeddingClient{
		url: url,
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// Classify sends text to the embedding service for classification.
// Returns the classification result or error if the service is unavailable.
func (ec *EmbeddingClient) Classify(text string) (*ClassifyResponse, error) {
	if ec == nil || ec.url == "" {
		return nil, fmt.Errorf("embedding client not configured")
	}

	// Prepare request
	reqBody, err := json.Marshal(ClassifyRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Send HTTP request
	req, err := http.NewRequest("POST", ec.url+"/classify", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ec.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result ClassifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// HealthCheck checks if the embedding service is healthy.
func (ec *EmbeddingClient) HealthCheck() error {
	if ec == nil || ec.url == "" {
		return fmt.Errorf("embedding client not configured")
	}

	req, err := http.NewRequest("GET", ec.url+"/health", nil)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}

	resp, err := ec.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unhealthy status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
