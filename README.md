# K8schedul8r

K8schedul8r is a Kubernetes scheduler that manages time-based scaling of workloads. It allows you to define time windows during which your deployments or statefulsets should run with different numbers of replicas.

## Features

- Time-based scheduling of Kubernetes workloads
- Support for both Deployments and StatefulSets
- Local and remote configuration options
- Simple YAML/JSON configuration format
- Immediate scaling with no gradual changes
- Automatic return to original replica count outside time windows

## Prerequisites

- K8 Cluster
- Go 1.23 or later
- Docker
- kubectl

## Note

- This project is still heavily under development and not ready for production use.

## Quick Start

1. **Build the Scheduler Image**

```bash
# Build the image
docker build -t k8schedul8r:latest .
```

2. **Deploy K8schedul8r**

```bash
# Apply RBAC and scheduler deployment
kubectl apply -f k8s-manifests.yaml
```

## Configuration

K8schedul8r uses a YAML configuration file to define scaling windows. Example:

```yaml
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
      - startTime: 1740261000  # Unix timestamp
        endTime: 1740264600    # Unix timestamp
        replicas: 4
```

### Configuration Fields

- `version`: Configuration version (currently "1")
- `resources`: List of resources to manage
  - `name`: Resource identifier
  - `namespace`: Kubernetes namespace
  - `target`: Resource to scale
    - `name`: Target resource name
    - `kind`: Resource type (Deployment/StatefulSet)
    - `apiVersion`: API version (usually apps/v1)
  - `originalReplicas`: Default replica count
  - `windows`: List of scaling windows
    - `startTime`: Window start (Unix timestamp)
    - `endTime`: Window end (Unix timestamp)
    - `replicas`: Desired replicas during window

## Important Notes

1. **Timestamps**
   - All times are in Unix timestamp format
   - Windows must not overlap
   - End time must be after start time
   - Past windows are allowed but will never be active

2. **Scaling Behavior**
   - Scaling is immediate (no gradual changes)
   - First matching window wins if there's overlap
   - Returns to originalReplicas when no window is active
   - Zero replicas is a valid configuration

3. **Common Pitfalls**
   - Ensure timestamps are in the future
   - Use correct field names (startTime/endTime, not start/end)
   - Configure proper RBAC permissions

## Development Setup

1. **Local Development Environment, Such as Minikube**
```bash
# Start Minikube
minikube start

# Configure Docker environment
eval $(minikube docker-env)

# Build the scheduler
docker build -t k8schedul8r:latest .
```

2. **Test Application Setup**
```bash
# Build test app
cd test-app
docker build -t hello-app:latest .

# Deploy test app
kubectl apply -f k8s-deployment.yaml
```

3. **Deploy Scheduler**
```bash
# Apply RBAC and configuration
kubectl apply -f k8s-manifests.yaml
```

## Monitoring

Monitor the scheduler:
```bash
# View scheduler logs
kubectl logs -l app=k8schedul8r -f

# Check deployment status
kubectl get deployment hello-app
```

## Architecture

- Runs as a single pod in your cluster
- Uses ServiceAccount for authentication
- Polls for configuration changes
- Directly interacts with Kubernetes API

## Future Enhancements

- TBD

## Contributing

Feel free to contribute to the project by opening issues or submitting pull requests.

## License

MIT