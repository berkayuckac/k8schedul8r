package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/berkayuckac/k8schedul8r/pkg/config"
	"github.com/berkayuckac/k8schedul8r/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Logger interface allows for custom logging implementations
type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

// stdLogger implements Logger using the standard log package
type stdLogger struct{}

func (l *stdLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *stdLogger) Println(v ...interface{}) {
	log.Println(v...)
}

// Scheduler manages the time-based scaling of resources
type Scheduler struct {
	provider     config.Provider
	pollInterval time.Duration
	stopCh       chan struct{}
	stopOnce     sync.Once
	logger       Logger
	client       kubernetes.Interface
	wg           sync.WaitGroup
}

// Options configures the scheduler behavior
type Options struct {
	// How often to check for scaling changes
	PollInterval time.Duration
	// Logger to use, if nil a standard logger will be used
	Logger Logger
	// Kubernetes client to use, if nil an in-cluster client will be created
	Client kubernetes.Interface
}

// New creates a new scheduler instance
func New(provider config.Provider, opts Options) (*Scheduler, error) {
	if opts.PollInterval == 0 {
		opts.PollInterval = 30 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = &stdLogger{}
	}

	var client kubernetes.Interface
	if opts.Client != nil {
		client = opts.Client
	} else {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
		}
	}

	return &Scheduler{
		provider:     provider,
		pollInterval: opts.PollInterval,
		stopCh:       make(chan struct{}),
		logger:       opts.Logger,
		client:       client,
	}, nil
}

// Start begins the scheduling loop
func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Printf("Starting scheduler with poll interval: %v", s.pollInterval)

	s.wg.Add(1)
	defer s.wg.Done()

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	// Do initial check immediately
	if err := s.checkAndScale(ctx); err != nil {
		s.logger.Printf("Initial scaling check failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Println("Context cancelled, stopping scheduler")
			return nil
		case <-s.stopCh:
			s.logger.Println("Stop signal received, stopping scheduler")
			return nil
		case <-ticker.C:
			if err := s.checkAndScale(ctx); err != nil {
				s.logger.Printf("Scaling check failed: %v", err)
			}
		}
	}
}

// Stop gracefully stops the scheduler and waits for all operations to complete
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		// If using a remote provider, stop it as well
		if remoteProvider, ok := s.provider.(*config.RemoteProvider); ok {
			remoteProvider.Stop()
		}
	})
	s.wg.Wait()
}

// checkAndScale performs a single check of all resources and applies scaling if needed
func (s *Scheduler) checkAndScale(ctx context.Context) error {
	// Load configuration
	resources, err := s.provider.Load(true)
	if err != nil {
		s.logger.Printf("Configuration load failed: %v", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if len(resources) == 0 {
		s.logger.Println("No resources loaded")
		return nil
	}

	now := time.Now().Unix()

	// Process each resource
	for _, res := range resources {
		desiredReplicas := res.GetDesiredReplicas(now)
		s.logger.Printf("Resource %s/%s: desired replicas: %d", res.Namespace, res.Name, desiredReplicas)

		if err := s.scaleResource(ctx, &res, desiredReplicas); err != nil {
			s.logger.Printf("Failed to scale %s/%s: %v", res.Namespace, res.Name, err)
			continue
		}

		s.logger.Printf("Successfully scaled %s %s/%s to %d replicas",
			res.Target.Kind, res.Namespace, res.Target.Name, desiredReplicas)
	}

	return nil
}

// scaleResource scales a kubernetes resource to the desired number of replicas
func (s *Scheduler) scaleResource(ctx context.Context, res *model.Resource, replicas int32) error {
	switch res.Target.Kind {
	case "Deployment":
		return s.scaleDeployment(ctx, res.Target.Name, res.Namespace, replicas)
	case "StatefulSet":
		return s.scaleStatefulSet(ctx, res.Target.Name, res.Namespace, replicas)
	default:
		return fmt.Errorf("unsupported resource kind: %s", res.Target.Kind)
	}
}

// scaleDeployment scales a deployment to the desired number of replicas
func (s *Scheduler) scaleDeployment(ctx context.Context, name, namespace string, replicas int32) error {
	deployment, err := s.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == replicas {
		s.logger.Printf("Deployment %s/%s already at %d replicas", namespace, name, replicas)
		return nil
	}

	// Create a copy of the deployment to modify
	deploymentCopy := deployment.DeepCopy()
	deploymentCopy.Spec.Replicas = &replicas

	_, err = s.client.AppsV1().Deployments(namespace).Update(ctx, deploymentCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	s.logger.Printf("Successfully scaled deployment %s/%s", namespace, name)
	return nil
}

// scaleStatefulSet scales a statefulset to the desired number of replicas
func (s *Scheduler) scaleStatefulSet(ctx context.Context, name, namespace string, replicas int32) error {
	statefulset, err := s.client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get statefulset: %w", err)
	}

	if statefulset.Spec.Replicas != nil && *statefulset.Spec.Replicas == replicas {
		s.logger.Printf("StatefulSet %s/%s already at %d replicas", namespace, name, replicas)
		return nil
	}

	// Create a copy of the statefulset to modify
	statefulsetCopy := statefulset.DeepCopy()
	statefulsetCopy.Spec.Replicas = &replicas

	_, err = s.client.AppsV1().StatefulSets(namespace).Update(ctx, statefulsetCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update statefulset: %w", err)
	}

	s.logger.Printf("Successfully scaled statefulset %s/%s to %d replicas", namespace, name, replicas)
	return nil
}
