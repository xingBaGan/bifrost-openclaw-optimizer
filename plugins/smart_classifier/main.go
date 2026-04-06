package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/maximhq/bifrost/core/schemas"
)

type SmartClassifierPlugin struct {
	logger schemas.Logger
}

func (p *SmartClassifierPlugin) GetName() string {
	return "smart-classifier"
}

func (p *SmartClassifierPlugin) Cleanup() error {
	return nil
}

func (p *SmartClassifierPlugin) HTTPTransportPreHook(ctx context.Context, req *http.Request) (*http.Request, error) {
	// Add routing logic
	model := req.Header.Get("X-Bifrost-Model")
	if model == "smart-route" {
		// Example: route to kimi
		req.Header.Set("X-Bifrost-Route-To", "kimi")
		if p.logger != nil {
			p.logger.Info("Smart Classifier: routing to kimi")
		}
	}
	return req, nil
}

func InitSmartClassifier(config any, logger schemas.Logger) (schemas.BasePlugin, error) {
	fmt.Println("Smart Classifier Plugin (In-package) Init")
	return &SmartClassifierPlugin{logger: logger}, nil
}
