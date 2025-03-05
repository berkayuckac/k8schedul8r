package model

import (
	"strings"
	"testing"
	"time"
)

func TestScalingWindow_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		window   ScalingWindow
		now      int64
		expected bool
	}{
		{
			name: "time is within window",
			window: ScalingWindow{
				StartTime: 100,
				EndTime:   200,
				Replicas:  3,
			},
			now:      150,
			expected: true,
		},
		{
			name: "time is before window",
			window: ScalingWindow{
				StartTime: 100,
				EndTime:   200,
				Replicas:  3,
			},
			now:      50,
			expected: false,
		},
		{
			name: "time is after window",
			window: ScalingWindow{
				StartTime: 100,
				EndTime:   200,
				Replicas:  3,
			},
			now:      250,
			expected: false,
		},
		{
			name: "time is at start boundary",
			window: ScalingWindow{
				StartTime: 100,
				EndTime:   200,
				Replicas:  3,
			},
			now:      100,
			expected: true,
		},
		{
			name: "time is at end boundary",
			window: ScalingWindow{
				StartTime: 100,
				EndTime:   200,
				Replicas:  3,
			},
			now:      200,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.window.IsActive(tt.now); got != tt.expected {
				t.Errorf("ScalingWindow.IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestScalingWindow_Validate(t *testing.T) {
	tests := []struct {
		name        string
		window      ScalingWindow
		wantErr     bool
		errContains string
	}{
		{
			name: "valid window",
			window: ScalingWindow{
				StartTime: 100,
				EndTime:   200,
				Replicas:  3,
			},
			wantErr: false,
		},
		{
			name: "start time after end time",
			window: ScalingWindow{
				StartTime: 200,
				EndTime:   100,
				Replicas:  3,
			},
			wantErr:     true,
			errContains: "start time must be before end time",
		},
		{
			name: "negative replicas",
			window: ScalingWindow{
				StartTime: 100,
				EndTime:   200,
				Replicas:  -1,
			},
			wantErr:     true,
			errContains: "replicas cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.window.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ScalingWindow.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("ScalingWindow.Validate() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestResource_GetDesiredReplicas(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name     string
		resource Resource
		now      int64
		want     int32
	}{
		{
			name: "no active windows returns original replicas",
			resource: Resource{
				Name:             "test-resource",
				OriginalReplicas: 2,
				Windows: []ScalingWindow{
					{
						StartTime: now + 100,
						EndTime:   now + 200,
						Replicas:  5,
					},
				},
			},
			now:  now,
			want: 2,
		},
		{
			name: "active window returns window replicas",
			resource: Resource{
				Name:             "test-resource",
				OriginalReplicas: 2,
				Windows: []ScalingWindow{
					{
						StartTime: now - 50,
						EndTime:   now + 50,
						Replicas:  5,
					},
				},
			},
			now:  now,
			want: 5,
		},
		{
			name: "multiple windows, first active window is used",
			resource: Resource{
				Name:             "test-resource",
				OriginalReplicas: 2,
				Windows: []ScalingWindow{
					{
						StartTime: now - 50,
						EndTime:   now + 50,
						Replicas:  5,
					},
					{
						StartTime: now - 40,
						EndTime:   now + 40,
						Replicas:  3,
					},
				},
			},
			now:  now,
			want: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resource.GetDesiredReplicas(tt.now); got != tt.want {
				t.Errorf("Resource.GetDesiredReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_Validate(t *testing.T) {
	tests := []struct {
		name        string
		resource    Resource
		wantErr     bool
		errContains string
	}{
		{
			name: "valid resource",
			resource: Resource{
				Name:      "test-resource",
				Namespace: "default",
				Target: Target{
					Name: "deployment-1",
					Kind: "Deployment",
				},
				OriginalReplicas: 2,
				Windows: []ScalingWindow{
					{
						StartTime: 100,
						EndTime:   200,
						Replicas:  3,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			resource: Resource{
				Namespace: "default",
				Target: Target{
					Name: "deployment-1",
					Kind: "Deployment",
				},
				OriginalReplicas: 2,
			},
			wantErr:     true,
			errContains: "resource name is required",
		},
		{
			name: "missing namespace",
			resource: Resource{
				Name: "test-resource",
				Target: Target{
					Name: "deployment-1",
					Kind: "Deployment",
				},
				OriginalReplicas: 2,
			},
			wantErr:     true,
			errContains: "namespace is required",
		},
		{
			name: "missing target name",
			resource: Resource{
				Name:      "test-resource",
				Namespace: "default",
				Target: Target{
					Kind: "Deployment",
				},
				OriginalReplicas: 2,
			},
			wantErr:     true,
			errContains: "target name is required",
		},
		{
			name: "missing target kind",
			resource: Resource{
				Name:      "test-resource",
				Namespace: "default",
				Target: Target{
					Name: "deployment-1",
				},
				OriginalReplicas: 2,
			},
			wantErr:     true,
			errContains: "target kind is required",
		},
		{
			name: "negative original replicas",
			resource: Resource{
				Name:      "test-resource",
				Namespace: "default",
				Target: Target{
					Name: "deployment-1",
					Kind: "Deployment",
				},
				OriginalReplicas: -1,
			},
			wantErr:     true,
			errContains: "original replicas cannot be negative",
		},
		{
			name: "invalid window",
			resource: Resource{
				Name:      "test-resource",
				Namespace: "default",
				Target: Target{
					Name: "deployment-1",
					Kind: "Deployment",
				},
				OriginalReplicas: 2,
				Windows: []ScalingWindow{
					{
						StartTime: 200,
						EndTime:   100,
						Replicas:  3,
					},
				},
			},
			wantErr:     true,
			errContains: "window 0 is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.resource.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Resource.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Resource.Validate() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
