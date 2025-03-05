package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLocalProvider_Load(t *testing.T) {
	now := time.Now().Unix()

	// Valid JSON configuration
	validJSON := `[
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
        "startTime": ` + fmt.Sprint(now) + `,
        "endTime": ` + fmt.Sprint(now+3600) + `,
        "replicas": 3
      }
    ]
  }
]`

	// Valid YAML configuration
	validYAML := `- name: test-scaler
  namespace: default
  target:
    name: test-deployment
    kind: Deployment
  originalReplicas: 2
  windows:
    - startTime: ` + fmt.Sprint(now) + `
      endTime: ` + fmt.Sprint(now+3600) + `
      replicas: 3`

	// Invalid configuration (empty array)
	invalidConfig := `[]`

	tests := []struct {
		name        string
		content     string
		ext         string
		validate    bool
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid yaml with validation",
			content:  validYAML,
			ext:      ".yaml",
			validate: true,
			wantErr:  false,
		},
		{
			name:     "valid yaml without validation",
			content:  validYAML,
			ext:      ".yml",
			validate: false,
			wantErr:  false,
		},
		{
			name:     "valid json with validation",
			content:  validJSON,
			ext:      ".json",
			validate: true,
			wantErr:  false,
		},
		{
			name:     "valid json without validation",
			content:  validJSON,
			ext:      ".json",
			validate: false,
			wantErr:  false,
		},
		{
			name:        "invalid config with validation",
			content:     invalidConfig,
			ext:         ".yaml",
			validate:    true,
			wantErr:     true,
			errContains: "no resources defined",
		},
		{
			name:     "invalid config without validation",
			content:  invalidConfig,
			ext:      ".yaml",
			validate: false,
			wantErr:  false,
		},
		{
			name:        "unsupported file format",
			content:     validYAML,
			ext:         ".txt",
			validate:    true,
			wantErr:     true,
			errContains: "unsupported file format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpfile, err := os.CreateTemp("", "test-config-*"+tt.ext)
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			// Write configuration to file
			if err := os.WriteFile(tmpfile.Name(), []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			// Create provider and load configuration
			provider := NewLocalProvider(tmpfile.Name())
			resources, err := provider.Load(tt.validate)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Validate loaded configuration
			if resources == nil {
				t.Error("resources is nil")
				return
			}

			// Skip resource count validation for invalid config without validation
			if tt.content == invalidConfig && !tt.validate {
				return
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
		})
	}
}

func TestLocalProvider_Load_FileNotFound(t *testing.T) {
	provider := NewLocalProvider("nonexistent.yaml")
	_, err := provider.Load(true)
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestLocalProvider_Load_InvalidJSON(t *testing.T) {
	// Create temporary file with invalid JSON
	tmpfile, err := os.CreateTemp("", "test-config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	invalidJSON := `{
		"name": "test-scaler",
		"namespace": "default",
		"target": {
			"name": "test-deployment",
			"kind": "Deployment"
		},
		invalid json here
	}`

	if err := os.WriteFile(tmpfile.Name(), []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	provider := NewLocalProvider(tmpfile.Name())
	_, err = provider.Load(true)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestLocalProvider_Load_InvalidYAML(t *testing.T) {
	// Create temporary file with invalid YAML
	tmpfile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	invalidYAML := `- name: test-scaler
  namespace: default
  target:
    name: test-deployment
    kind: Deployment
  originalReplicas: invalid-number  # Invalid number value
  windows:
    - startTime: "not-a-timestamp"  # Invalid time value
      endTime: "also-invalid"
      replicas: "not-a-number"`

	if err := os.WriteFile(tmpfile.Name(), []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	provider := NewLocalProvider(tmpfile.Name())
	_, err = provider.Load(true)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}
