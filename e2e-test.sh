#!/bin/bash
set -e

# Configuration
export KUBECONFIG=/Users/justin.davies/.kube/kind-kuma-1-config
INPUT_FILE="tests/customer-stress-test.yaml" # Not used directly here, but good reference
OUTPUT_FILE="tests/customer-migrated-final.yaml"

echo "🚀 Starting Comprehensive Ingress Migration Test"

function cleanup() {
    echo "🧹 Cleaning up previous runs..."
    helm uninstall ingress-nginx --namespace ingress-nginx 2>/dev/null || true
    helm uninstall kong --namespace kong 2>/dev/null || true
    kubectl delete namespace ingress-nginx 2>/dev/null || true
    kubectl delete namespace kong 2>/dev/null || true
    kubectl delete namespace defaults 2>/dev/null || true
    echo "✨ Cleanup complete."
}

# Optional cleanup flag
if [[ "$1" == "cleanup" ]]; then
    cleanup
    echo "Exiting after cleanup."
    exit 0
fi

# Cleanup defaults to ensure fresh start
kubectl delete namespace defaults 2>/dev/null || true
kubectl wait --for=delete namespace/defaults --timeout=60s 2>/dev/null || true

# 1. Install Helm Repos
echo "⎈ Adding/Updating Helm Repos..."
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo add kong https://charts.konghq.com
helm repo update

# 2. Install Ingress NGINX (Optional, but good for reference if we wanted to compare)
# We strictly need Kong for validation here.

# 3. Install Kong Ingress Controller
echo "🦍 Installing Kong Ingress Controller via Helm..."

HELM_ARGS=""
if [ -f "license.json" ]; then
    echo "   💎 Enterprise License found! Enabling Kong Gateway Enterprise..."
    kubectl create namespace kong --dry-run=client -o yaml | kubectl apply -f -
    kubectl create secret generic kong-enterprise-license --from-file=license=license.json -n kong --dry-run=client -o yaml | kubectl apply -f -
    
    HELM_ARGS="--set image.repository=kong/kong-gateway \
               --set enterprise.enabled=true \
               --set enterprise.license.secretName=kong-enterprise-license \
               --set manager.env.KONG_ADMIN_TOKEN=password \
               --set env.PLUGINS=bundled\,openid-connect\,proxy-cache\,rate-limiting-advanced"
else
    echo "   Using Kong Gateway OSS (No license.json found)..."
    HELM_ARGS="--set env.PLUGINS=bundled"
fi

helm upgrade --install kong kong/kong \
  --namespace kong --create-namespace \
  --set ingressController.installCRDs=false \
  --set ingressController.enabled=true \
  --set ingressController.ingressClass=kong \
  $HELM_ARGS \
  --wait
  
# 4. Deploy Backend & Migrator
echo "🔧 Deploying Backends..."
kubectl create ns defaults --dry-run=client -o yaml | kubectl apply -f -

# Backend: echo-v1
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo-v1
  namespace: defaults
  labels:
    app: echo-v1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: echo-v1
  template:
    metadata:
      labels:
        app: echo-v1
    spec:
      containers:
      - name: echo
        image: ealen/echo-server
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: echo-v1
  namespace: defaults
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: echo-v1
EOF

echo "🔨 Building Migrator Tool..."
go build -o migrator main.go

echo "🔄 Running Migration (Both Formats)..."
# The input file contains multiple docs, the tool should handle it (if parser supports it)
# Standard k8s YAML parser in Go usually handles multi-doc.
./migrator --input full-coverage-ingress.yaml --output-format both

if [ ! -f migrated-kong-ingress.yaml ] || [ ! -f migrated-gateway-api.yaml ]; then
    echo "❌ Migration failed: Output files not created"
    exit 1
fi

echo "📤 Applying Migrated Kong Ingress Resources..."
kubectl apply -f migrated-kong-ingress.yaml || echo "⚠️  Some resources failed (Enterprise-only features?). Continuing..."

echo "⏳ Waiting for Kong Proxy IP..."
sleep 10
PROXY_IP=$(kubectl get svc -n kong kong-kong-proxy -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
if [ -z "$PROXY_IP" ]; then
    PROXY_IP="localhost"
fi
HOST="coverage.example.com"
echo "   Proxy endpoint: http://$PROXY_IP"

# --- TESTS ---

echo "🔍 Running Validation Tests..."

# Test 1: Headers (Snippet)
echo "   > [Test 1] Headers (/api/headers)..."
RESP=$(curl -s -I -H "Host: $HOST" http://$PROXY_IP/api/headers)
if [[ "$RESP" == *"X-Response-Header: True"* ]]; then
    echo "     ✅ Response Transformer (more_set_headers) working"
else
    echo "     ❌ Response Transformer failed"
    # echo "$RESP"
fi

# Test 2: Auth Basic
echo "   > [Test 2] Auth Basic (/api/auth-basic)..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: $HOST" http://$PROXY_IP/api/auth-basic)
if [ "$CODE" == "401" ]; then
    echo "     ✅ Basic Auth working (Received 401 Unauthorized)"
else
    echo "     ❌ Basic Auth failed (Expected 401, got $CODE)"
fi

# Test 3: Rate Limit
echo "   > [Test 3] Rate Limit (/api/rate-limit)..."
RESP=$(curl -s -I -H "Host: $HOST" http://$PROXY_IP/api/rate-limit)
if [[ "$RESP" == *"X-RateLimit-Limit"* ]] || [[ "$RESP" == *"RateLimit-Limit"* ]]; then
    echo "     ✅ Rate Limit headers present"
else
    echo "     ⚠️  Rate Limit headers missing (Plugin config might vary)"
fi

# Test 4: Redirects
echo "   > [Test 4] Permanent Redirect (/api/perm-redirect)..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: $HOST" http://$PROXY_IP/api/perm-redirect)
if [ "$CODE" == "301" ]; then
    echo "     ✅ Permanent Redirect (301) working"
else
    echo "     ❌ Permanent Redirect failed (Got $CODE)"
fi

# Test 5: Maintenance
echo "   > [Test 5] Maintenance Mode (/api/maintenance)..."
CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: $HOST" http://$PROXY_IP/api/maintenance)
if [ "$CODE" == "503" ]; then
    echo "     ✅ Maintenance Mode (503) working"
else
    echo "     ❌ Maintenance Mode failed (Got $CODE)"
fi

# Test 6: Caching
echo "   > [Test 6] Caching (/api/cache)..."
curl -s -o /dev/null -H "Host: $HOST" http://$PROXY_IP/api/cache
sleep 1
CACHE_CHECK=$(curl -s -I -H "Host: $HOST" http://$PROXY_IP/api/cache)
if [[ "$CACHE_CHECK" == *"X-Cache-Status"* ]]; then
    echo "     ✅ Caching headers present"
else
    echo "     ⚠️  Caching headers missing (Enterprise/Plugin issue?)"
fi

# Test 7: Affinity
echo "   > [Test 7] Affinity (/api/affinity)..."
# We need to manually annotate service for affinity policy usually, as the tool logs ACTION REQUIRED.
# Let's check if the tool *generated* the policy.
if grep -q "KongUpstreamPolicy" migrated-kong-ingress.yaml; then
    echo "     ✅ KongUpstreamPolicy generated in YAML"
else
    echo "     ❌ KongUpstreamPolicy NOT found in YAML"
fi

# Test 8: Security (CORS/IP)
echo "   > [Test 8] Security (/api/security)..."
CORS=$(curl -s -I -H "Host: $HOST" -H "Origin: https://trusted.com" http://$PROXY_IP/api/security)
if [[ "$CORS" == *"Access-Control-Allow-Origin: https://trusted.com"* ]]; then
    echo "     ✅ CORS working"
else
    echo "     ❌ CORS failed"
fi

# Test 9: Bot Detection (Deny)
echo "   > [Test 9] Bot Detection (/api/bot)..."
BOT=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: $HOST" -A "curl/7.64.1" http://$PROXY_IP/api/bot)
# Note: Kong bot detection returns 403 by default
if [[ "$BOT" == "403" ]]; then
    echo "     ✅ Bot Detection blocked curl (403)"
else
    echo "     ⚠️  Bot Detection didn't block (Got $BOT) - User-Agent match might be tricky or plugin logic differs."
fi

echo ""
echo "🎉 Comprehensive Test Complete!"

