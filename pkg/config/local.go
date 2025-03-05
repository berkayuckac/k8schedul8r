package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/berkayuckac/k8schedul8r/pkg/model"
	"gopkg.in/yaml.v3"
)

type LocalProvider struct {
	path string
}

func NewLocalProvider(path string) *LocalProvider {
	return &LocalProvider{
		path: path,
	}
}

// Load implements Provider.Load
func (l *LocalProvider) Load(validate bool) ([]model.Resource, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var resources []model.Resource
	ext := strings.ToLower(filepath.Ext(l.path))

	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &resources)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		err = json.Unmarshal(data, &resources)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}

	if validate {
		if len(resources) == 0 {
			return nil, fmt.Errorf("no resources defined")
		}
		for i, res := range resources {
			if err := res.Validate(); err != nil {
				return nil, fmt.Errorf("resource[%d] validation failed: %w", i, err)
			}
		}
	}

	return resources, nil
}
