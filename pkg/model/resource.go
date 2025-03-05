package model

import (
	"fmt"
)

// Resource represents a Kubernetes resource with time-based scaling configuration
type Resource struct {
	// Name of the resource
	Name string `json:"name" yaml:"name"`
	// Namespace where the resource is located
	Namespace string `json:"namespace" yaml:"namespace"`
	// Target defines the resource to be scaled
	Target Target `json:"target" yaml:"target"`
	// OriginalReplicas is the base number of replicas to return to when no window is active
	OriginalReplicas int32 `json:"originalReplicas" yaml:"originalReplicas"`
	// Windows defines the time windows for scaling
	Windows []ScalingWindow `json:"windows" yaml:"windows"`
}

// Target defines the Kubernetes resource to be scaled
type Target struct {
	// Name of the target resource
	Name string `json:"name" yaml:"name"`
	// Kind of the target resource ("Deployment" OR "StatefulSet")
	Kind string `json:"kind" yaml:"kind"`
	// APIVersion of the target resource
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
}

// ScalingWindow defines a time window for scaling
type ScalingWindow struct {
	StartTime int64 `json:"startTime" yaml:"startTime"`
	EndTime   int64 `json:"endTime" yaml:"endTime"`
	Replicas  int32 `json:"replicas" yaml:"replicas"`
}

func (w *ScalingWindow) IsActive(now int64) bool {
	return now >= w.StartTime && now < w.EndTime
}

func (w *ScalingWindow) Validate() error {
	if w.StartTime >= w.EndTime {
		return fmt.Errorf("start time must be before end time")
	}
	if w.Replicas < 0 {
		return fmt.Errorf("replicas cannot be negative")
	}
	return nil
}

func (r *Resource) GetDesiredReplicas(now int64) int32 {
	for _, window := range r.Windows {
		if window.IsActive(now) {
			return window.Replicas
		}
	}
	return r.OriginalReplicas
}

func (r *Resource) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("resource name is required")
	}
	if r.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if r.Target.Name == "" {
		return fmt.Errorf("target name is required")
	}
	if r.Target.Kind == "" {
		return fmt.Errorf("target kind is required")
	}
	if r.OriginalReplicas < 0 {
		return fmt.Errorf("original replicas cannot be negative")
	}

	// Validate all windows
	for i, window := range r.Windows {
		if err := window.Validate(); err != nil {
			return fmt.Errorf("window %d is invalid: %w", i, err)
		}
	}

	return nil
}
