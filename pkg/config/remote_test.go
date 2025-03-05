package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRemoteProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  RemoteConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: RemoteConfig{
				URL:          "http://example.com",
				PollInterval: time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing url",
			config: RemoteConfig{
				PollInterval: time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero poll interval",
			config: RemoteConfig{
				URL:          "http://example.com",
				PollInterval: 0,
			},
			wantErr: true,
		},
		{
			name: "negative poll interval",
			config: RemoteConfig{
				URL:          "http://example.com",
				PollInterval: -time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewRemoteProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRemoteProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("NewRemoteProvider() returned nil provider")
			}
		})
	}
}

func TestRemoteProvider_Load(t *testing.T) {
	now := time.Now().Unix()

	validConfig := fmt.Sprintf(`[
  {
    "name": "test-scaler",
    "namespace": "default",
    "target": {
      "name": "test-deployment",
      "kind": "Deployment"
    },
    "originalReplicas": 2,
    "windows": [
      {
        "startTime": %d,
        "endTime": %d,
        "replicas": 3
      }
    ]
  }
]`, now, now+3600)

	tests := []struct {
		name         string
		config       RemoteConfig
		validate     bool
		setupServer  func() *httptest.Server
		wantErr      bool
		errContains  string
		checkContent bool
	}{
		{
			name: "valid yaml response",
			config: RemoteConfig{
				PollInterval: time.Second,
			},
			validate: true,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/yaml")
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, validConfig)
				}))
			},
			wantErr:      false,
			checkContent: true,
		},
		{
			name: "valid json response",
			config: RemoteConfig{
				PollInterval: time.Second,
			},
			validate: true,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, validConfig)
				}))
			},
			wantErr:      false,
			checkContent: true,
		},
		{
			name: "server error",
			config: RemoteConfig{
				PollInterval: time.Second,
			},
			validate: true,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, "internal error")
				}))
			},
			wantErr:     true,
			errContains: "500",
		},
		{
			name: "invalid yaml response",
			config: RemoteConfig{
				PollInterval: time.Second,
			},
			validate: true,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/yaml")
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, "invalid: yaml: content: {")
				}))
			},
			wantErr:     true,
			errContains: "failed to parse YAML",
		},
		{
			name: "with bearer token",
			config: RemoteConfig{
				PollInterval: time.Second,
				BearerToken:  "test-token",
			},
			validate: true,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					auth := r.Header.Get("Authorization")
					if auth != "Bearer test-token" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "application/yaml")
					w.WriteHeader(http.StatusOK)
					fmt.Fprint(w, validConfig)
				}))
			},
			wantErr:      false,
			checkContent: true,
		},
		{
			name: "unauthorized",
			config: RemoteConfig{
				PollInterval: time.Second,
				BearerToken:  "wrong-token",
			},
			validate: true,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
					fmt.Fprint(w, "unauthorized")
				}))
			},
			wantErr:     true,
			errContains: "401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			tt.config.URL = server.URL
			if strings.Contains(tt.name, "yaml") {
				tt.config.URL = server.URL + "/config.yaml"
			} else if strings.Contains(tt.name, "json") {
				tt.config.URL = server.URL + "/config.json"
			}

			provider, err := NewRemoteProvider(tt.config)
			if err != nil {
				t.Fatalf("failed to create provider: %v", err)
			}

			resources, err := provider.Load(tt.validate)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if tt.checkContent {
				if resources == nil {
					t.Fatal("resources is nil")
				}
				if len(resources) != 1 {
					t.Errorf("expected 1 resource, got %d", len(resources))
					return
				}

				resource := resources[0]
				if resource.Name != "test-scaler" {
					t.Errorf("expected resource name test-scaler, got %s", resource.Name)
				}
				if resource.Namespace != "default" {
					t.Errorf("expected namespace default, got %s", resource.Namespace)
				}
				if resource.Target.Name != "test-deployment" {
					t.Errorf("expected target name test-deployment, got %s", resource.Target.Name)
				}
				if resource.Target.Kind != "Deployment" {
					t.Errorf("expected target kind Deployment, got %s", resource.Target.Kind)
				}
				if resource.OriginalReplicas != 2 {
					t.Errorf("expected originalReplicas 2, got %d", resource.OriginalReplicas)
				}
				if len(resource.Windows) != 1 {
					t.Errorf("expected 1 window, got %d", len(resource.Windows))
					return
				}

				window := resource.Windows[0]
				if window.StartTime != now {
					t.Errorf("expected startTime %d, got %d", now, window.StartTime)
				}
				if window.EndTime != now+3600 {
					t.Errorf("expected endTime %d, got %d", now+3600, window.EndTime)
				}
				if window.Replicas != 3 {
					t.Errorf("expected replicas 3, got %d", window.Replicas)
				}
			}
		})
	}
}

func TestRemoteProvider_Load_Caching(t *testing.T) {
	now := time.Now().Unix()
	validConfig := fmt.Sprintf(`[
  {
    "name": "test-scaler",
    "namespace": "default",
    "target": {
      "name": "test-deployment",
      "kind": "Deployment"
    },
    "originalReplicas": 2,
    "windows": [
      {
        "startTime": %d,
        "endTime": %d,
        "replicas": 3
      }
    ]
  }
]`, now, now+3600)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validConfig)
	}))
	defer server.Close()

	config := RemoteConfig{
		URL:          server.URL,
		PollInterval: time.Second,
	}

	provider, err := NewRemoteProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// First request should fetch from server
	resources1, err := provider.Load(true)
	if err != nil {
		t.Fatalf("first Load() failed: %v", err)
	}

	// Second request within cache TTL should use cached value
	resources2, err := provider.Load(true)
	if err != nil {
		t.Fatalf("second Load() failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 request to server, got %d", requestCount)
	}

	if len(resources1) != 1 || len(resources2) != 1 {
		t.Error("expected both loads to return 1 resource")
	}
}

func TestRemoteProvider_BackgroundPolling(t *testing.T) {
	now := time.Now().Unix()
	validConfig := fmt.Sprintf(`[
  {
    "name": "test-scaler",
    "namespace": "default",
    "target": {
      "name": "test-deployment",
      "kind": "Deployment"
    },
    "originalReplicas": 2,
    "windows": [
      {
        "startTime": %d,
        "endTime": %d,
        "replicas": 3
      }
    ]
  }
]`, now, now+3600)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, validConfig)
	}))
	defer server.Close()

	config := RemoteConfig{
		URL:          server.URL,
		PollInterval: 100 * time.Millisecond,
	}

	provider, err := NewRemoteProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Wait for a few polling intervals
	time.Sleep(350 * time.Millisecond)

	// Stop the provider
	provider.Stop()

	// Wait a bit more to ensure no more requests are made
	time.Sleep(200 * time.Millisecond)

	// We should have seen approximately 4 requests (initial + 3 polls)
	// Allow for some timing variation
	if requestCount < 3 || requestCount > 5 {
		t.Errorf("expected 3-5 requests, got %d", requestCount)
	}
}
