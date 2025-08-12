#!/bin/bash

# GCP Setup Script for Weather Service
# This script configures GCP after authentication

set -e

echo "🚀 GCP Setup for Weather Service"
echo "================================"

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo "❌ gcloud CLI is not installed. Please install it first."
    exit 1
fi

# Check if authenticated
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" &> /dev/null; then
    echo "❌ Not authenticated with GCP. Please run:"
    echo "   gcloud auth login"
    exit 1
fi

echo "✅ gcloud CLI found and authenticated"

# Function to prompt for input
prompt_with_default() {
    local prompt=$1
    local default=$2
    local var_name=$3
    
    read -p "$prompt [$default]: " input
    eval "$var_name=\${input:-$default}"
}

# Get project configuration
echo ""
echo "📋 Available GCP Projects:"
gcloud projects list --format="table(projectId,name,projectNumber)"

echo ""
prompt_with_default "Enter your GCP Project ID" "" PROJECT_ID

if [ -z "$PROJECT_ID" ]; then
    echo "❌ Project ID is required"
    exit 1
fi

prompt_with_default "Enter preferred region" "us-central1" REGION
prompt_with_default "Enter preferred zone" "us-central1-a" ZONE
prompt_with_default "Enter GKE cluster name" "weather-service-cluster" CLUSTER_NAME

# Set project configuration
echo ""
echo "🔧 Configuring GCP project..."
gcloud config set project $PROJECT_ID
gcloud config set compute/region $REGION
gcloud config set compute/zone $ZONE

# Enable required APIs
echo ""
echo "🔌 Enabling required APIs..."
APIS=(
    "compute.googleapis.com"
    "container.googleapis.com"
    "cloudbuild.googleapis.com"
    "containerregistry.googleapis.com"
    "artifactregistry.googleapis.com"
    "cloudtrace.googleapis.com"
    "cloudprofiler.googleapis.com"
    "monitoring.googleapis.com"
    "logging.googleapis.com"
    "sqladmin.googleapis.com"
)

for api in "${APIS[@]}"; do
    echo "  Enabling $api..."
    gcloud services enable $api --quiet
done

# Create Artifact Registry repository
echo ""
echo "📦 Setting up Artifact Registry..."
REPO_NAME="weather-service"
REPO_LOCATION=$REGION

if ! gcloud artifacts repositories describe $REPO_NAME --location=$REPO_LOCATION &> /dev/null; then
    echo "  Creating Artifact Registry repository..."
    gcloud artifacts repositories create $REPO_NAME \
        --repository-format=docker \
        --location=$REPO_LOCATION \
        --description="Docker repository for weather service"
else
    echo "  Repository $REPO_NAME already exists"
fi

# Check if cluster exists
echo ""
echo "☸️  Checking for GKE cluster..."
if gcloud container clusters describe $CLUSTER_NAME --zone=$ZONE &> /dev/null; then
    echo "  Cluster $CLUSTER_NAME already exists"
    prompt_with_default "Do you want to use the existing cluster? (y/n)" "y" USE_EXISTING
    
    if [ "$USE_EXISTING" != "y" ]; then
        prompt_with_default "Create a new cluster with a different name?" "weather-service-new" NEW_CLUSTER_NAME
        CLUSTER_NAME=$NEW_CLUSTER_NAME
    fi
fi

# Create GKE cluster if it doesn't exist
if ! gcloud container clusters describe $CLUSTER_NAME --zone=$ZONE &> /dev/null; then
    echo ""
    echo "🏗️  Creating GKE cluster..."
    prompt_with_default "Number of nodes" "3" NUM_NODES
    prompt_with_default "Machine type" "e2-medium" MACHINE_TYPE
    
    gcloud container clusters create $CLUSTER_NAME \
        --zone $ZONE \
        --num-nodes $NUM_NODES \
        --machine-type $MACHINE_TYPE \
        --enable-autoscaling \
        --min-nodes 1 \
        --max-nodes 5 \
        --enable-autorepair \
        --enable-autoupgrade \
        --release-channel stable \
        --enable-stackdriver-kubernetes \
        --addons HorizontalPodAutoscaling,HttpLoadBalancing,GcePersistentDiskCsiDriver
fi

# Get cluster credentials
echo ""
echo "🔑 Getting cluster credentials..."
gcloud container clusters get-credentials $CLUSTER_NAME --zone=$ZONE

# Set up Cloud Build permissions
echo ""
echo "🔐 Setting up Cloud Build permissions..."
PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format='value(projectNumber)')

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member=serviceAccount:$PROJECT_NUMBER@cloudbuild.gserviceaccount.com \
    --role=roles/container.developer \
    --quiet

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member=serviceAccount:$PROJECT_NUMBER@cloudbuild.gserviceaccount.com \
    --role=roles/artifactregistry.writer \
    --quiet

# Create Cloud SQL instance (optional)
echo ""
prompt_with_default "Do you want to create a Cloud SQL PostgreSQL instance? (y/n)" "n" CREATE_SQL

if [ "$CREATE_SQL" == "y" ]; then
    echo "🗄️  Creating Cloud SQL instance..."
    INSTANCE_NAME="weather-service-db"
    
    if ! gcloud sql instances describe $INSTANCE_NAME &> /dev/null; then
        gcloud sql instances create $INSTANCE_NAME \
            --database-version=POSTGRES_14 \
            --tier=db-f1-micro \
            --region=$REGION \
            --network=default \
            --backup \
            --backup-start-time=03:00
        
        # Create database
        gcloud sql databases create weather_service --instance=$INSTANCE_NAME
        
        # Set password for postgres user
        gcloud sql users set-password postgres \
            --instance=$INSTANCE_NAME \
            --password=weather123
    else
        echo "  SQL instance $INSTANCE_NAME already exists"
    fi
fi

# Update cloudbuild.yaml with correct values
echo ""
echo "📝 Updating cloudbuild.yaml..."
sed -i.bak \
    -e "s/YOUR_PROJECT_ID/$PROJECT_ID/g" \
    -e "s/YOUR_CLUSTER_NAME/$CLUSTER_NAME/g" \
    -e "s/YOUR_CLUSTER_ZONE/$ZONE/g" \
    -e "s|gcr.io/YOUR_PROJECT_ID|$REPO_LOCATION-docker.pkg.dev/$PROJECT_ID/$REPO_NAME|g" \
    ../cloudbuild.yaml

# Update Kubernetes manifests
echo ""
echo "📝 Updating Kubernetes manifests..."
find ../k8s -name "*.yaml" -type f -exec sed -i.bak \
    -e "s|gcr.io/YOUR_PROJECT_ID|$REPO_LOCATION-docker.pkg.dev/$PROJECT_ID/$REPO_NAME|g" \
    {} \;

# Create namespace in cluster
echo ""
echo "📁 Creating Kubernetes namespace..."
kubectl create namespace weather-service --dry-run=client -o yaml | kubectl apply -f -

# Apply ConfigMap
echo ""
echo "⚙️  Applying ConfigMap..."
kubectl apply -f ../k8s/base/configmap.yaml -n weather-service

echo ""
echo "✅ GCP Setup Complete!"
echo ""
echo "📋 Configuration Summary:"
echo "  Project ID: $PROJECT_ID"
echo "  Region: $REGION"
echo "  Zone: $ZONE"
echo "  GKE Cluster: $CLUSTER_NAME"
echo "  Artifact Registry: $REPO_LOCATION-docker.pkg.dev/$PROJECT_ID/$REPO_NAME"
echo ""
echo "🚀 Next Steps:"
echo "  1. Deploy using Cloud Build:"
echo "     cd .. && gcloud builds submit --config cloudbuild.yaml ."
echo ""
echo "  2. Or deploy manually:"
echo "     # Build and push Docker image"
echo "     docker build -t $REPO_LOCATION-docker.pkg.dev/$PROJECT_ID/$REPO_NAME/weather-service:latest ."
echo "     docker push $REPO_LOCATION-docker.pkg.dev/$PROJECT_ID/$REPO_NAME/weather-service:latest"
echo ""
echo "     # Apply Kubernetes manifests"
echo "     kubectl apply -f k8s/base/ -n weather-service"
echo ""
echo "  3. Get service endpoint:"
echo "     kubectl get service weather-service -n weather-service"
echo ""
echo "  4. View logs:"
echo "     kubectl logs -f deployment/weather-service -n weather-service"
echo ""
echo "  5. Access Grafana (if deployed):"
echo "     kubectl port-forward -n monitoring svc/grafana 3000:3000"