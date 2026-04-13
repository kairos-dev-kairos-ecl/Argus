package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// LocalModelConnector handles connections to local model servers (Ollama, vLLM, etc)
type LocalModelConnector struct {
	serverURL  string
	modelName  string
	logger     *zap.Logger
	httpClient *http.Client
	lastCheck  time.Time
	healthy    bool
}

// NewLocalModelConnector creates a new local model connector
func NewLocalModelConnector(serverURL string, modelName string, logger *zap.Logger) *LocalModelConnector {
	return &LocalModelConnector{
		serverURL: serverURL,
		modelName: modelName,
		logger:    logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		healthy: false,
	}
}

// DiscoverModels lists available models from the server
func (c *LocalModelConnector) DiscoverModels(ctx context.Context) ([]string, error) {
	// Try Ollama endpoint first
	models, err := c.discoverOllama(ctx)
	if err == nil && len(models) > 0 {
		c.logger.Info("discovered models via Ollama", zap.Strings("models", models))
		return models, nil
	}

	// Try vLLM endpoint
	models, err = c.discoverVLLM(ctx)
	if err == nil && len(models) > 0 {
		c.logger.Info("discovered models via vLLM", zap.Strings("models", models))
		return models, nil
	}

	// Try generic endpoint
	models, err = c.discoverGeneric(ctx)
	if err == nil && len(models) > 0 {
		c.logger.Info("discovered models via generic", zap.Strings("models", models))
		return models, nil
	}

	c.logger.Error("failed to discover models from server", zap.String("server", c.serverURL))
	return nil, fmt.Errorf("failed to discover models from %s", c.serverURL)
}

// discoverOllama queries Ollama's /api/tags endpoint
func (c *LocalModelConnector) discoverOllama(ctx context.Context) ([]string, error) {
	resp, err := c.httpClient.Get(c.serverURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama endpoint returned status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Models {
		models = append(models, m.Name)
	}

	return models, nil
}

// discoverVLLM queries vLLM's /v1/models endpoint
func (c *LocalModelConnector) discoverVLLM(ctx context.Context) ([]string, error) {
	resp, err := c.httpClient.Get(c.serverURL + "/v1/models")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vLLM endpoint returned status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

// discoverGeneric queries a generic /models endpoint
func (c *LocalModelConnector) discoverGeneric(ctx context.Context) ([]string, error) {
	resp, err := c.httpClient.Get(c.serverURL + "/models")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("generic endpoint returned status %d", resp.StatusCode)
	}

	var result struct {
		Models []string `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Models, nil
}

// HealthCheck verifies connectivity to the model server
func (c *LocalModelConnector) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.serverURL+"/health", nil)
	if err != nil {
		c.healthy = false
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.healthy = false
		c.logger.Warn("health check failed", zap.String("server", c.serverURL), zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.healthy = false
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	c.healthy = true
	c.lastCheck = time.Now()
	c.logger.Debug("health check passed", zap.String("server", c.serverURL))
	return nil
}

// IsHealthy returns the current health status
func (c *LocalModelConnector) IsHealthy() bool {
	// Consider unhealthy if last check was more than 5 minutes ago
	if time.Since(c.lastCheck) > 5*time.Minute {
		return false
	}
	return c.healthy
}

// ExtractSignals parses inference response for signal extraction
func (c *LocalModelConnector) ExtractSignals(resp map[string]interface{}, depth string) map[string]interface{} {
	signals := make(map[string]interface{})

	// Always extract basic info
	signals["model"] = c.modelName
	signals["extracted_at"] = time.Now().Unix()

	// Token counts
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		if promptTokens, ok := usage["prompt_tokens"].(float64); ok {
			signals["tokens_input"] = int(promptTokens)
		}
		if completionTokens, ok := usage["completion_tokens"].(float64); ok {
			signals["tokens_output"] = int(completionTokens)
		}
	}

	// Full extraction if requested
	if depth == "full" {
		// Logprobs
		if logprobs, ok := resp["logprobs"].(map[string]interface{}); ok {
			signals["logprobs"] = logprobs
		}

		// Sampling parameters
		if choices, ok := resp["choices"].([]interface{}); ok {
			for i, choice := range choices {
				if choiceMap, ok := choice.(map[string]interface{}); ok {
					signals[fmt.Sprintf("choice_%d_finish_reason", i)] = choiceMap["finish_reason"]
					if finishDetails, ok := choiceMap["finish_details"].(map[string]interface{}); ok {
						signals[fmt.Sprintf("choice_%d_finish_details", i)] = finishDetails
					}
				}
			}
		}
	}

	return signals
}

// StartHealthCheckLoop runs periodic health checks
func (c *LocalModelConnector) StartHealthCheckLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial check
	c.HealthCheck(ctx)

	for {
		select {
		case <-ticker.C:
			if err := c.HealthCheck(ctx); err != nil {
				c.logger.Warn("periodic health check failed",
					zap.String("server", c.serverURL),
					zap.Error(err),
				)
			}
		case <-ctx.Done():
			c.logger.Info("stopping health check loop", zap.String("server", c.serverURL))
			return
		}
	}
}
