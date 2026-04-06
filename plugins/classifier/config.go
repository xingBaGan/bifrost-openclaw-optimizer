package server

// ClassifierConfig is the configuration for the classifier plugin.
type ClassifierConfig struct {
	EmbeddingService *EmbeddingServiceConfig `json:"embedding_service,omitempty"`
}

// EmbeddingServiceConfig configures the semantic-router embedding service.
type EmbeddingServiceConfig struct {
	Enabled             bool    `json:"enabled"`
	URL                 string  `json:"url"`
	TimeoutMs           int     `json:"timeout_ms"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
	FallbackToRules     bool    `json:"fallback_to_rules"`
}
