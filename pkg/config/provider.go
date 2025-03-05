package config

import "github.com/berkayuckac/k8schedul8r/pkg/model"

// Provider defines the interface for configuration providers
type Provider interface {
	// If validate is true, the configuration will be validated before being returned
	Load(validate bool) ([]model.Resource, error)
}
