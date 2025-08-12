# GCP Setup Instructions

## 1. Authenticate with Google Cloud

Open this URL in your browser to authenticate:
```
https://accounts.google.com/o/oauth2/auth?response_type=code&client_id=32555940559.apps.googleusercontent.com&redirect_uri=https%3A%2F%2Fsdk.cloud.google.com%2Fauthcode.html&scope=openid+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.email+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fappengine.admin+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fsqlservice.login+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcompute+https%3A%2F%2Fwww.googleapis.com%2Fauth%2Faccounts.reauth&state=zxRDnwF1TVqcstNRziI4p4cKZh86pE&prompt=consent&token_usage=remote&access_type=offline&code_challenge=DcfSX9d4Qpj5M8dKIEVdPC-Car7TciMEbZislLcjI3E&code_challenge_method=S256
```

After authenticating, you'll receive a verification code. Run this command with your code:
```bash
gcloud auth login --verification-code=YOUR_CODE_HERE
```

## 2. Set up your project

Once authenticated, configure your project:
```bash
# List available projects
gcloud projects list

# Set your project (replace with your actual project ID)
gcloud config set project YOUR_PROJECT_ID

# Set default region and zone
gcloud config set compute/region us-central1
gcloud config set compute/zone us-central1-a
```

## 3. Enable required APIs

```bash
# Enable necessary APIs for the weather service
gcloud services enable compute.googleapis.com
gcloud services enable container.googleapis.com
gcloud services enable cloudbuild.googleapis.com
gcloud services enable containerregistry.googleapis.com
gcloud services enable cloudtrace.googleapis.com
gcloud services enable cloudprofiler.googleapis.com
gcloud services enable monitoring.googleapis.com
gcloud services enable logging.googleapis.com
```

## 4. Create a GKE cluster (if needed)

```bash
# Create a GKE cluster for deployment
gcloud container clusters create weather-service-cluster \
    --zone us-central1-a \
    --num-nodes 3 \
    --machine-type e2-medium \
    --enable-autoscaling \
    --min-nodes 1 \
    --max-nodes 5 \
    --enable-autorepair \
    --enable-autoupgrade \
    --release-channel stable
```

## 5. Configure kubectl for GKE

```bash
# Get cluster credentials
gcloud container clusters get-credentials weather-service-cluster \
    --zone us-central1-a
```

## 6. Set up Cloud Build

```bash
# Grant Cloud Build permission to deploy to GKE
export PROJECT_ID=$(gcloud config get-value project)
export PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format='value(projectNumber)')

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member=serviceAccount:$PROJECT_NUMBER@cloudbuild.gserviceaccount.com \
    --role=roles/container.developer
```

## 7. Deploy using Cloud Build

```bash
# Submit build to Cloud Build
gcloud builds submit --config cloudbuild.yaml .
```

## Alternative: Manual authentication

If you prefer, you can also use:
```bash
gcloud auth application-default login
```

This will authenticate you for application default credentials.