package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/berkayuckac/k8schedul8r/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// testLogger captures log output for testing
type testLogger struct {
	mu      sync.Mutex
	entries []string
}

func newTestLogger() *testLogger {
	return &testLogger{
		entries: make([]string, 0),
	}
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprintf(format, v...))
}

func (l *testLogger) Println(v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprint(v...))
}

// TODO and note to self: Should these tests rely on the logger this much?
func (l *testLogger) getEntries() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.entries...)
}

// mockProvider implements config.Provider for testing
type mockProvider struct {
	resources []model.Resource
	err       error
	mu        sync.Mutex
	loads     int // count how many times Load was called
}

func (m *mockProvider) Load(validate bool) ([]model.Resource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loads++
	return m.resources, m.err
}

func (m *mockProvider) getLoadCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loads
}

func createTestDeployment(name, namespace string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name         string
		pollInterval time.Duration
		logger       Logger
		want         time.Duration
		wantErr      bool
	}{
		{
			name:         "uses default interval when zero",
			pollInterval: 0,
			want:         30 * time.Second,
		},
		{
			name:         "uses provided interval",
			pollInterval: 5 * time.Second,
			want:         5 * time.Second,
		},
		{
			name:         "uses provided logger",
			pollInterval: time.Second,
			logger:       newTestLogger(),
			want:         time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockProvider{}
			client := fake.NewSimpleClientset()
			s, err := New(provider, Options{
				PollInterval: tt.pollInterval,
				Logger:       tt.logger,
				Client:       client,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if s.pollInterval != tt.want {
					t.Errorf("New() pollInterval = %v, want %v", s.pollInterval, tt.want)
				}

				if tt.logger != nil && s.logger != tt.logger {
					t.Error("New() logger not set correctly")
				}
			}
		})
	}
}

func TestScheduler_Start(t *testing.T) {
	now := time.Now().Unix()
	testResources := []model.Resource{
		{
			Name:      "test-scaler",
			Namespace: "default",
			Target: model.Target{
				Name: "test-deployment",
				Kind: "Deployment",
			},
			OriginalReplicas: 2,
			Windows: []model.ScalingWindow{
				{
					StartTime: now - 3600, // 1 hour ago
					EndTime:   now + 3600, // 1 hour from now
					Replicas:  5,
				},
			},
		},
	}

	// Create test deployment
	deployment := createTestDeployment("test-deployment", "default", 2)
	client := fake.NewSimpleClientset(deployment)

	tests := []struct {
		name           string
		resources      []model.Resource
		providerErr    error
		pollInterval   time.Duration
		runDuration    time.Duration
		wantMinChecks  int
		wantLogEntries []string
	}{
		{
			name:          "performs initial check and periodic checks",
			resources:     testResources,
			pollInterval:  100 * time.Millisecond,
			runDuration:   250 * time.Millisecond,
			wantMinChecks: 3, // Initial check + at least 2 periodic checks
			wantLogEntries: []string{
				"Starting scheduler",
				"desired replicas: 5",
				"Successfully scaled deployment",
			},
		},
		{
			name:          "handles provider errors gracefully",
			resources:     testResources,
			providerErr:   fmt.Errorf("provider error"),
			pollInterval:  100 * time.Millisecond,
			runDuration:   250 * time.Millisecond,
			wantMinChecks: 3,
			wantLogEntries: []string{
				"Starting scheduler",
				"Configuration load failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockProvider{
				resources: tt.resources,
				err:       tt.providerErr,
			}

			logger := newTestLogger()
			s, err := New(provider, Options{
				PollInterval: tt.pollInterval,
				Logger:       logger,
				Client:       client,
			})
			if err != nil {
				t.Fatalf("Failed to create scheduler: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), tt.runDuration)
			defer cancel()

			// Start scheduler in a goroutine
			done := make(chan struct{})
			go func() {
				s.Start(ctx)
				close(done)
			}()

			// Wait for either context timeout or scheduler to stop
			select {
			case <-ctx.Done():
				// Expected case - timeout reached
			case <-done:
				if ctx.Err() == nil {
					t.Error("Scheduler stopped unexpectedly")
				}
			}

			// Verify log entries
			entries := logger.getEntries()
			for _, want := range tt.wantLogEntries {
				found := false
				for _, entry := range entries {
					if strings.Contains(entry, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Log entry not found: %s", want)
				}
			}

			// Verify minimum number of checks
			if provider.getLoadCount() < tt.wantMinChecks {
				t.Errorf("Expected at least %d checks, got %d", tt.wantMinChecks, provider.getLoadCount())
			}
		})
	}
}

func TestScheduler_Stop(t *testing.T) {
	provider := &mockProvider{}
	client := fake.NewSimpleClientset()
	s, err := New(provider, Options{
		PollInterval: time.Second,
		Client:       client,
	})
	if err != nil {
		t.Fatalf("Failed to create scheduler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scheduler in a goroutine
	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	// Wait a bit to ensure scheduler is running
	time.Sleep(100 * time.Millisecond)

	// Stop the scheduler
	s.Stop()

	// Wait for scheduler to stop
	select {
	case <-done:
		// Expected case
	case <-time.After(time.Second):
		t.Error("Scheduler did not stop within timeout")
	}
}

func TestScheduler_ConcurrentAccess(t *testing.T) {
	provider := &mockProvider{}
	client := fake.NewSimpleClientset()
	s, err := New(provider, Options{
		PollInterval: time.Second,
		Client:       client,
	})
	if err != nil {
		t.Fatalf("Failed to create scheduler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scheduler in a goroutine
	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	// Wait a bit to ensure scheduler is running
	time.Sleep(100 * time.Millisecond)

	// Stop the scheduler multiple times concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Stop()
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Wait for scheduler to stop
	select {
	case <-done:
		// Expected case
	case <-time.After(time.Second):
		t.Error("Scheduler did not stop within timeout")
	}
}

func TestScheduler_checkAndScale(t *testing.T) {
	now := time.Now().Unix()
	testResources := []model.Resource{
		{
			Name:      "test-scaler",
			Namespace: "default",
			Target: model.Target{
				Name: "test-deployment",
				Kind: "Deployment",
			},
			OriginalReplicas: 2,
			Windows: []model.ScalingWindow{
				{
					StartTime: now - 3600, // 1 hour ago
					EndTime:   now + 3600, // 1 hour from now
					Replicas:  5,
				},
			},
		},
	}

	// Create test deployment
	deployment := createTestDeployment("test-deployment", "default", 2)
	client := fake.NewSimpleClientset(deployment)

	tests := []struct {
		name           string
		resources      []model.Resource
		providerErr    error
		wantLogEntries []string
	}{
		{
			name:      "scales deployment successfully",
			resources: testResources,
			wantLogEntries: []string{
				"desired replicas: 5",
				"Successfully scaled deployment",
			},
		},
		{
			name:        "handles provider errors",
			resources:   testResources,
			providerErr: fmt.Errorf("provider error"),
			wantLogEntries: []string{
				"Configuration load failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockProvider{
				resources: tt.resources,
				err:       tt.providerErr,
			}

			logger := newTestLogger()
			s, err := New(provider, Options{
				PollInterval: time.Second,
				Logger:       logger,
				Client:       client,
			})
			if err != nil {
				t.Fatalf("Failed to create scheduler: %v", err)
			}

			ctx := context.Background()
			err = s.checkAndScale(ctx)

			if tt.providerErr != nil {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify log entries
			entries := logger.getEntries()
			for _, want := range tt.wantLogEntries {
				found := false
				for _, entry := range entries {
					if strings.Contains(entry, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Log entry not found: %s", want)
				}
			}
		})
	}
}
