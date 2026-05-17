#!/bin/bash

# SecureStream Local K8s Setup Script

echo "🚀 Starting SecureStream Local Kubernetes Setup..."

# 1. Build Images
echo "📦 Building Docker images..."
docker build -t securestream-backend:local ./backend
docker build -t securestream-frontend:local ./frontend
docker build -t securestream-simulator:local ./client-simulator

# 2. Setup Namespace
echo "🌐 Creating namespace..."
kubectl apply -f k8s/base/common.yaml

# 3. Handle Secrets
echo "🔑 Please enter your GROQ_API_KEY (leave empty to skip):"
read -r GROQ_KEY

if [ ! -z "$GROQ_KEY" ]; then
    ENCODED_KEY=$(echo -n "$GROQ_KEY" | base64)
    # Update the secret in the cluster directly
    kubectl patch secret securestream-secrets -n securestream -p "{\"data\":{\"GROQ_API_KEY\":\"$ENCODED_KEY\"}}"
    echo "✅ GROQ_API_KEY updated."
else
    echo "⚠️  Skipping GROQ_API_KEY update."
fi

# 4. Deploy Infrastructure
echo "🗄️  Deploying Postgres and Redis..."
kubectl apply -f k8s/base/postgres.yaml
kubectl apply -f k8s/base/redis.yaml

# 5. Deploy Application
echo "🚢 Deploying Application (Local Overlay)..."
kubectl apply -k k8s/local

echo "⏳ Waiting for pods to be ready..."
kubectl get pods -n securestream -w
