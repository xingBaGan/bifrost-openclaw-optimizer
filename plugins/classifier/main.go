package server

import (
	"fmt"

	"github.com/maximhq/bifrost/core/schemas"
)

// Global instance of the plugin (for .so loading)
var pluginInstance *ClassifierPlugin

// InitLegacy is a remnant of a previous build attempt.
func InitLegacy(config any) error {
	return nil
}

// GetName returns the canonical name "classifier", which MUST match config.json.
func GetName() string {
	return "classifier"
}

// HTTPTransportPreHook is the main entry point for request classification.
func HTTPTransportPreHook(ctx *schemas.BifrostContext, req *schemas.HTTPRequest) (*schemas.HTTPResponse, error) {
	if pluginInstance == nil {
		return nil, nil
	}
	return pluginInstance.HTTPTransportPreHook(ctx, req)
}

// HTTPTransportPostHook is not used by the classifier.
func HTTPTransportPostHook(
	ctx *schemas.BifrostContext, req *schemas.HTTPRequest, resp *schemas.HTTPResponse,
) error {
	if pluginInstance == nil {
		return nil
	}
	return pluginInstance.HTTPTransportPostHook(ctx, req, resp)
}

// PreLLMHook is optional.
func PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	return req, nil, nil
}

// PostLLMHook is optional.
func PostLLMHook(ctx *schemas.BifrostContext, resp *schemas.BifrostResponse, bifrostErr *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	return resp, bifrostErr, nil
}

// Cleanup is called on shutdown.
func Cleanup() error {
	if pluginInstance == nil {
		return nil
	}
	return pluginInstance.Cleanup()
}
