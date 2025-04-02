package config

import (
	"fmt"

	"github.com/berkayuckac/k8schedul8r/pkg/model"
)

type MultiProvider struct {
	providers []Provider
}

func NewMultiProvider(providers ...Provider) *MultiProvider {
	return &MultiProvider{
		providers: providers,
	}
}

// Load implements Provider interface
func (m *MultiProvider) Load(validate bool) ([]model.Resource, error) {
	var allResources []model.Resource

	for _, provider := range m.providers {
		resources, err := provider.Load(validate)
		if err != nil {
			return nil, fmt.Errorf("failed to load from provider: %w", err)
		}
		allResources = append(allResources, resources...)
	}

	return allResources, nil
}
