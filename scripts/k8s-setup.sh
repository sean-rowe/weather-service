#!/bin/bash

# Kubernetes setup script using kind (Kubernetes in Docker)
set -e

echo "🚀 Setting up local Kubernetes cluster with kind..."

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo -e "${RED}❌ kind is not installed${NC}"
    echo "Installing kind..."
    
    # Detect OS
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        if command -v brew &> /dev/null; then
            brew install kind
        else
            echo "Please install kind manually: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
            exit 1
        fi
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux
        curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
        chmod +x ./kind
        sudo mv ./kind /usr/local/bin/kind
    else
        echo "Unsupported OS. Please install kind manually: https://kind.sigs.k8s.io/"
        exit 1
    fi
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo -e "${YELLOW}⚠ kubectl is not installed. Installing...${NC}"
    
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install kubectl
    else
        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
        chmod +x kubectl
        sudo mv kubectl /usr/local/bin/
    fi
fi

# Create kind cluster configuration
cat <<EOF > kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: weather-service
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 8080
    protocol: TCP
  - containerPort: 30090
    hostPort: 9090
    protocol: TCP
EOF

# Check if cluster already exists
if kind get clusters | grep -q weather-service; then
    echo -e "${YELLOW}Cluster 'weather-service' already exists${NC}"
    read -p "Delete and recreate? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        kind delete cluster --name weather-service
    else
        echo "Using existing cluster"
    fi
fi

# Create kind cluster
echo -e "\n${YELLOW}Creating kind cluster...${NC}"
kind create cluster --config kind-config.yaml

# Wait for cluster to be ready
echo -e "\n${YELLOW}Waiting for cluster to be ready...${NC}"
kubectl wait --for=condition=Ready nodes --all --timeout=60s

# Install metrics server
echo -e "\n${YELLOW}Installing metrics server...${NC}"
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Create namespace
echo -e "\n${YELLOW}Creating weather-service namespace...${NC}"
kubectl create namespace weather-service --dry-run=client -o yaml | kubectl apply -f -

# Install Prometheus Operator (optional - for ServiceMonitor support)
echo -e "\n${YELLOW}Installing Prometheus Operator...${NC}"
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

# Build and load Docker image into kind
echo -e "\n${YELLOW}Building and loading Docker image...${NC}"
docker build -t weather-service:latest .
kind load docker-image weather-service:latest --name weather-service

# Update image in deployment to use local image
sed -i.bak 's|gcr.io/PROJECT_ID/weather-service:latest|weather-service:latest|g' k8s/base/deployment.yaml
sed -i.bak 's|imagePullPolicy: Always|imagePullPolicy: IfNotPresent|g' k8s/base/deployment.yaml

# Apply Kubernetes manifests
echo -e "\n${YELLOW}Applying Kubernetes manifests...${NC}"
kubectl apply -f k8s/base/ -n weather-service

# Wait for deployment to be ready
echo -e "\n${YELLOW}Waiting for deployment to be ready...${NC}"
kubectl wait --for=condition=available --timeout=120s deployment/weather-service -n weather-service

# Get pod status
echo -e "\n${GREEN}Deployment status:${NC}"
kubectl get pods -n weather-service
kubectl get services -n weather-service

echo -e "\n${GREEN}✅ Kubernetes setup complete!${NC}"
echo ""
echo "Access the service:"
echo "  Weather Service: http://localhost:8080"
echo ""
echo "Useful commands:"
echo "  kubectl get pods -n weather-service"
echo "  kubectl logs -f deployment/weather-service -n weather-service"
echo "  kubectl port-forward service/weather-service 8080:80 -n weather-service"
echo "  kubectl exec -it deployment/weather-service -n weather-service -- /bin/sh"
echo ""
echo "To delete the cluster:"
echo "  kind delete cluster --name weather-service"

# Cleanup
rm -f kind-config.yaml
rm -f k8s/base/deployment.yaml.bak