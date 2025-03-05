package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/berkayuckac/k8schedul8r/pkg/model"
	"gopkg.in/yaml.v3"
)

// RemoteConfig holds the configuration for the remote provider
type RemoteConfig struct {
	URL          string        `json:"url" yaml:"url"`
	PollInterval time.Duration `json:"pollInterval" yaml:"pollInterval"`
	BearerToken  string        `json:"bearerToken" yaml:"bearerToken"`
}

// cachedConfig holds a configuration with its metadata
type cachedConfig struct {
	resources []model.Resource
	fetchedAt time.Time
}

// RemoteProvider implements Provider interface for remote HTTP configurations
type RemoteProvider struct {
	config     RemoteConfig
	httpClient *http.Client
	cache      *cachedConfig
	cacheMu    sync.RWMutex
	stopCh     chan struct{}
	stopped    bool
	stoppedMu  sync.RWMutex
	wg         sync.WaitGroup
}

func NewRemoteProvider(config RemoteConfig) (*RemoteProvider, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}

	if config.PollInterval <= 0 {
		return nil, fmt.Errorf("poll interval must be positive")
	}

	provider := &RemoteProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // reasonable default timeout, TODO configurable?
		},
		stopCh: make(chan struct{}),
	}

	// Start background polling
	go provider.pollConfig()

	return provider, nil
}

// Stop stops the background polling and waits for it to complete
func (r *RemoteProvider) Stop() {
	r.stoppedMu.Lock()
	defer r.stoppedMu.Unlock()

	if !r.stopped {
		close(r.stopCh)
		r.stopped = true
		r.wg.Wait()
	}
}

// pollConfig continuously polls the remote endpoint for configuration updates
func (r *RemoteProvider) pollConfig() {
	r.wg.Add(1)
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			if resources, err := r.fetchConfig(true); err == nil {
				r.updateCache(resources)
			}
		}
	}
}

// updateCache updates the cached configuration
func (r *RemoteProvider) updateCache(resources []model.Resource) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	r.cache = &cachedConfig{
		resources: resources,
		fetchedAt: time.Now(),
	}
}

// fetchConfig fetches the configuration from the remote endpoint
func (r *RemoteProvider) fetchConfig(validate bool) ([]model.Resource, error) {
	req, err := http.NewRequest(http.MethodGet, r.config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add bearer token if configured
	if r.config.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.config.BearerToken)
	}

	// Set Accept header based on URL extension
	if strings.HasSuffix(r.config.URL, ".yaml") || strings.HasSuffix(r.config.URL, ".yml") {
		req.Header.Set("Accept", "application/yaml")
	} else {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var resources []model.Resource

	// Try to determine the content type from the response
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "yaml") || strings.Contains(contentType, "yml") ||
		strings.HasSuffix(r.config.URL, ".yaml") || strings.HasSuffix(r.config.URL, ".yml") {
		if err := yaml.Unmarshal(body, &resources); err != nil {
			return nil, fmt.Errorf("failed to parse YAML configuration: %w", err)
		}
	} else {
		if err := json.Unmarshal(body, &resources); err != nil {
			return nil, fmt.Errorf("failed to parse JSON configuration: %w", err)
		}
	}

	if validate {
		for i, res := range resources {
			if err := res.Validate(); err != nil {
				return nil, fmt.Errorf("resource[%d] validation failed: %w", i, err)
			}
		}
	}

	return resources, nil
}

// Load implements Provider.Load
func (r *RemoteProvider) Load(validate bool) ([]model.Resource, error) {
	r.cacheMu.RLock()
	cache := r.cache
	r.cacheMu.RUnlock()

	// If we have a valid cache, return it
	if cache != nil && time.Since(cache.fetchedAt) < r.config.PollInterval {
		return cache.resources, nil
	}

	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	// Double-check cache under write lock
	if r.cache != nil && time.Since(r.cache.fetchedAt) < r.config.PollInterval {
		return r.cache.resources, nil
	}

	// Try to fetch new config
	resources, err := r.fetchConfig(validate)
	if err != nil {
		// On error, try to return cached config if available
		if r.cache != nil {
			return r.cache.resources, nil
		}
		return nil, err
	}

	r.cache = &cachedConfig{
		resources: resources,
		fetchedAt: time.Now(),
	}
	return resources, nil
}
