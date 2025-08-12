#!/bin/bash

# Setup script for local development environment
set -e

echo "🚀 Setting up Weather Service local development environment..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check prerequisites
check_command() {
    if ! command -v $1 &> /dev/null; then
        echo -e "${RED}❌ $1 is not installed${NC}"
        echo "Please install $1 and try again"
        exit 1
    else
        echo -e "${GREEN}✓ $1 is installed${NC}"
    fi
}

echo "Checking prerequisites..."
check_command go
check_command docker
check_command make

# Check Docker is running
if ! docker info &> /dev/null; then
    echo -e "${RED}❌ Docker is not running${NC}"
    echo "Please start Docker and try again"
    exit 1
else
    echo -e "${GREEN}✓ Docker is running${NC}"
fi

# Install Go dependencies
echo -e "\n${YELLOW}Installing Go dependencies...${NC}"
go mod download
go mod tidy

# Build the application
echo -e "\n${YELLOW}Building application...${NC}"
make build

# Create necessary directories
echo -e "\n${YELLOW}Creating directories...${NC}"
mkdir -p monitoring/grafana/dashboards
mkdir -p monitoring/prometheus
mkdir -p monitoring/otel
mkdir -p scripts
mkdir -p k8s/{base,staging,production}

# Pull Docker images
echo -e "\n${YELLOW}Pulling Docker images...${NC}"
docker pull postgres:15-alpine
docker pull prom/prometheus:latest
docker pull grafana/grafana:latest
docker pull jaegertracing/all-in-one:latest
docker pull otel/opentelemetry-collector-contrib:latest

# Check if kind is installed for Kubernetes
if command -v kind &> /dev/null; then
    echo -e "${GREEN}✓ kind is installed - Kubernetes deployment available${NC}"
else
    echo -e "${YELLOW}⚠ kind is not installed - Kubernetes deployment not available${NC}"
    echo "Install kind with: brew install kind (macOS) or check https://kind.sigs.k8s.io/"
fi

echo -e "\n${GREEN}✅ Setup complete!${NC}"
echo ""
echo "Available commands:"
echo "  make run-all      - Start all services (PostgreSQL, monitoring, app)"
echo "  make compose-up   - Start Docker Compose stack"
echo "  make run          - Run service locally without Docker"
echo "  make test         - Run tests"
echo "  make help         - Show all available commands"
echo ""
echo "Quick start:"
echo "  make run-all"
echo ""
echo "Then access:"
echo "  Weather Service: http://localhost:8080"
echo "  Prometheus:      http://localhost:9090"
echo "  Grafana:         http://localhost:3000 (admin/admin)"
echo "  Jaeger:          http://localhost:16686"