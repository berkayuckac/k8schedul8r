# K8schedul8r

K8schedul8r is a Kubernetes scheduler that manages time-based scaling of workloads. It allows you to define time windows during which your deployments or statefulsets should run with different numbers of replicas.

### Key Features

- Scale Kubernetes Deployments and StatefulSets based on time windows
- Multiple configuration options:
  - Kubernetes Custom Resources
  - ConfigMap-based configuration
  - Remote HTTP configuration
- Immediate scaling (no gradual changes)
- K8-native integration with RBAC and events
- Leader election for high availability

## Getting Started

### Prerequisites

- K8s
- kubectl
- Go
- Docker

### Installation

1. Clone the repository:
```bash
git clone https://github.com/berkayuckac/k8schedul8r.git
cd k8schedul8r
```

2. Build the controller:
```bash
# For Minikube
eval $(minikube docker-env)

# Build
docker build -t k8schedul8r:latest .
```

3. Deploy to Kubernetes:
```bash
kubectl apply -f k8s-manifests.yaml
```

## Usage

K8schedul8r supports three ways to configure scaling schedules:

### 1. Using Custom Resources

Create a `ScheduledResource` that defines when to scale your workload:

```yaml
apiVersion: k8schedul8r.io/v1alpha1
kind: ScheduledResource
metadata:
  name: my-app-schedule
  namespace: default
spec:
  target:
    name: my-app
    kind: Deployment
    apiVersion: apps/v1
  originalReplicas: 2  # Default replicas when no window is active
  windows:
    - startTime: 1743542601  # Unix timestamp
      endTime: 1743542661    # Unix timestamp
      replicas: 4            # Scale to 4 replicas during this window
```

Apply it:
```bash
kubectl apply -f my-schedule.yaml
```

### 2. Using ConfigMap

Create a ConfigMap with your scaling configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: k8schedul8r-config
  namespace: default
data:
  config.yaml: |
    version: "1"
    resources:
      - name: my-app
        namespace: default
        target:
          name: my-app
          kind: Deployment
          apiVersion: apps/v1
        originalReplicas: 2
        windows:
          - startTime: 1743542601
            endTime: 1743542661
            replicas: 4
```

Enable ConfigMap configuration in the deployment:
```yaml
args:
- --enable-config-file=true
- --config=/etc/k8schedul8r/config.yaml
```

### 3. Using Remote Configuration

Point K8schedul8r to a remote HTTP endpoint:

```yaml
args:
- --enable-remote-config=true
- --remote-config=http://config-server/scaling-config
```

The endpoint should return configuration in the same format as the ConfigMap.

## Configuration Options

### Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| --enable-crd-provider | Use CRD-based configuration | false |
| --enable-config-file | Use local file configuration | false |
| --enable-remote-config | Use remote HTTP configuration | false |
| --config | Path to config file | "" |
| --remote-config | URL for remote config | "" |
| --interval | Polling interval | 30s |
| --leader-elect | Enable leader election | false |

### Time Windows

- Use Unix timestamps for start/end times
- Windows must not overlap
- First matching window takes precedence
- Returns to originalReplicas when no window is active

## Monitoring

### View Controller Logs
```bash
kubectl logs -l app=k8schedul8r -f
```

### Check Scaling Events
```bash
# For CRD-based configuration
kubectl get events --field-selector involvedObject.kind=ScheduledResource

# Check target deployment
kubectl get deployment my-app
```

## Development

### Local Development Setup

1. Start Minikube:
```bash
minikube start
```

2. Build and deploy:
```bash
eval $(minikube docker-env)
docker build -t k8schedul8r:latest .
kubectl apply -f k8s-manifests.yaml
```

3. Test with example app:
```bash
kubectl create deployment hello-app --image=nginx:latest --replicas=2
kubectl apply -f examples/scheduled-resource.yaml
```

### Running Tests
```bash
go test ./... -v
```

## Architecture

- Built on controller-runtime framework
- Event-driven reconciliation for CRDs
- Pluggable configuration providers
- Leader election for HA deployments
- Kubernetes native integration (RBAC, events)

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Submit a Pull Request

## License

MIT