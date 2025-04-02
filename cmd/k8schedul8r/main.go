package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/berkayuckac/k8schedul8r/pkg/config"
	"github.com/berkayuckac/k8schedul8r/pkg/model"
	"github.com/berkayuckac/k8schedul8r/pkg/operator"
	"github.com/berkayuckac/k8schedul8r/pkg/scheduler"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(model.AddToScheme(scheme))
}

func main() {
	var (
		configPath         = flag.String("config", "", "Path to configuration file (optional)")
		remoteConfigURL    = flag.String("remote-config", "", "URL for remote configuration (optional)")
		pollInterval       = flag.Duration("interval", 30*time.Second, "How often to check for scaling changes")
		enableLeaderElect  = flag.Bool("leader-elect", false, "Enable leader election for controller manager.")
		enableConfigFile   = flag.Bool("enable-config-file", false, "Enable configuration from file.")
		enableCRDProvider  = flag.Bool("enable-crd-provider", false, "Enable CRD-based configuration.")
		enableRemoteConfig = flag.Bool("enable-remote-config", false, "Enable remote configuration fetching.")
		namespace          = flag.String("namespace", "default", "Namespace to watch for ScheduledResources")
	)
	flag.Parse()

	// Create the controller manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   *enableLeaderElect,
		LeaderElectionID: "k8schedul8r-leader",
	})
	if err != nil {
		log.Fatalf("Unable to start manager: %v", err)
	}

	var providers []config.Provider

	// Add file-based configuration if enabled
	if *enableConfigFile {
		if *configPath != "" {
			providers = append(providers, config.NewLocalProvider(*configPath))
			log.Printf("Enabled local config provider with path: %s", *configPath)
		} else {
			log.Println("Local config enabled but no path provided, skipping")
		}
	}

	// Add remote configuration if enabled
	if *enableRemoteConfig {
		if *remoteConfigURL != "" {
			remoteProvider, err := config.NewRemoteProvider(config.RemoteConfig{
				URL:          *remoteConfigURL,
				PollInterval: *pollInterval,
			})
			if err != nil {
				log.Printf("Warning: Failed to create remote provider: %v", err)
			} else {
				providers = append(providers, remoteProvider)
				log.Printf("Enabled remote config provider with URL: %s", *remoteConfigURL)
			}
		} else {
			log.Println("Remote config enabled but no URL provided, skipping")
		}
	}

	// Add CRD-based configuration if enabled
	var crdProvider *config.CRDProvider
	if *enableCRDProvider {
		crdConfig := config.CRDConfig{
			Namespace: *namespace,
		}
		var err error
		crdProvider, err = config.NewCRDProvider(crdConfig, mgr.GetClient(), mgr.GetScheme())
		if err != nil {
			log.Printf("Warning: Failed to create CRD provider: %v", err)
		} else {
			providers = append(providers, crdProvider)
			log.Printf("Enabled CRD config provider in namespace: %s", *namespace)
		}
	}

	// Create multi-provider if we have multiple providers
	var provider config.Provider
	switch len(providers) {
	case 0:
		log.Fatal("No configuration providers enabled. Enable at least one provider using --enable-config-file, --enable-crd-provider, or --enable-remote-config")
	case 1:
		provider = providers[0]
	default:
		provider = config.NewMultiProvider(providers...)
		log.Printf("Using %d configuration providers", len(providers))
	}

	// Create the scheduler
	sched, err := scheduler.New(provider, scheduler.Options{
		PollInterval: *pollInterval,
	})
	if err != nil {
		log.Fatalf("Failed to create scheduler: %v", err)
	}

	// Set up the controller if using CRD provider
	if *enableCRDProvider {
		if err = (&operator.ScheduledResourceReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr, sched, crdProvider); err != nil {
			log.Fatalf("Unable to create controller: %v", err)
		}
	}

	// Handle shutdown signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, initiating shutdown", sig)
		cancel()
		sched.Stop()
	}()

	// Start both the controller manager and scheduler
	go func() {
		log.Println("Starting scheduler")
		if err := sched.Start(ctx); err != nil {
			log.Fatalf("Scheduler failed: %v", err)
		}
	}()

	log.Println("Starting manager")
	if err := mgr.Start(ctx); err != nil {
		log.Fatalf("Manager failed: %v", err)
	}
}
