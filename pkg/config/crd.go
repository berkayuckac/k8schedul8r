package config

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/berkayuckac/k8schedul8r/pkg/model"
)

type CRDConfig struct {
	Namespace     string `json:"namespace" yaml:"namespace"`
	LabelSelector string `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
}

type CRDProvider struct {
	config CRDConfig
	client client.Client
	scheme *runtime.Scheme
	cache  *sync.Map
}

func NewCRDProvider(config CRDConfig, client client.Client, scheme *runtime.Scheme) (*CRDProvider, error) {
	provider := &CRDProvider{
		config: config,
		client: client,
		scheme: scheme,
		cache:  &sync.Map{},
	}

	return provider, nil
}

func (c *CRDProvider) UpdateResource(resource model.Resource) {
	key := fmt.Sprintf("%s/%s", resource.Namespace, resource.Name)
	c.cache.Store(key, resource)
}

func (c *CRDProvider) DeleteResource(namespace, name string) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	c.cache.Delete(key)
}

// Load implements Provider.Load
func (c *CRDProvider) Load(validate bool) ([]model.Resource, error) {
	var resources []model.Resource

	c.cache.Range(func(key, value interface{}) bool {
		if resource, ok := value.(model.Resource); ok {
			if validate {
				if err := resource.Validate(); err != nil {
					return false
				}
			}
			resources = append(resources, resource)
		}
		return true
	})

	return resources, nil
}
