# NGINX to Kong Ingress Migration Tool

This tool automates the migration of Kubernetes Ingress resources from **NGINX Ingress Controller** to **Kong Ingress Controller (KIC)**. It parses existing NGINX annotations and translates them into Kong-equivalent configurations, generating `Ingress`, `KongPlugin`, and `KongUpstreamPolicy` resources.

## Features

- **Automated Translation**: Converts NGINX annotations to Kong Plugins and CRDs.
- **Modern Kong Support**: targeting `KongUpstreamPolicy` for upstream configurations (replacing deprecated `KongIngress`).
- **Full Coverage Testing**: Includes an End-to-End (E2E) test suite validating the migration against a live cluster.
- **Safe Defaults**: Generates standard configuration while warning about incompatible or manual-intervention items.

## UI Dashboard

The migration tool includes a web-based dashboard that connects to your cluster and lets you browse, analyse, and migrate Ingress resources interactively.

### Run locally with binary

Download the binary from [the releases page](https://github.com/Kong/nginx-kong-migrator/releases), then start the dashboard:

```bash
./migrator ui
```

Optional flags:
- `-port <int>` — port to listen on (default: `8080`)
- `-namespace <string>` — restrict to a single namespace (default: all namespaces)
- `-kubeconfig <string>` — path to kubeconfig (falls back to `$KUBECONFIG` or `~/.kube/config`)

Example:
```bash
./migrator ui -port 9090 -namespace kong
```

Open [http://localhost:8080](http://localhost:8080) in your browser. The dashboard reads Ingress resources from your currently active cluster context and colour-codes each one:

- **Green** — already migrated (non-NGINX ingress)
- **Yellow** — ready to migrate, may have followup notes
- **Red** — has unmigrated annotations that require manual intervention

From the dashboard you can:
- **Migrate Selected / Migrate Single** — applies the migration in-cluster: sets `ingressClassName: kong`, creates `KongPlugin` and `KongUpstreamPolicy` CRDs
- **Copy to Namespace** — copies one or more ingresses (with their Kong resources) to a target namespace
- **Download as Kong Ingress YAML / Gateway API YAML** — export the migrated manifests without applying them
- **Refresh** — reload the ingress list from the cluster

### Run locally with Docker (out-of-cluster)

You can run the dashboard as a Docker container outside the cluster by mounting your kubeconfig:

```bash
docker run --rm \
  -p 8080:8080 \
  -e KUBECONFIG=/kubeconfig \
  -v "${KUBECONFIG:-$HOME/.kube/config}:/kubeconfig:ro" \
  kong/nginx-kong-migrator:latest ui
```

Then open [http://localhost:8080](http://localhost:8080). The container reads the mounted kubeconfig to connect to whichever cluster context is currently active.

To target a specific context, set it before running:

```bash
kubectl config use-context <your-context>
docker run --rm \
  -p 8080:8080 \
  -e KUBECONFIG=/kubeconfig \
  -v "${KUBECONFIG:-$HOME/.kube/config}:/kubeconfig:ro" \
  kong/nginx-kong-migrator:latest ui
```

### Deploy to Kubernetes

The `deploy/kubernetes.yaml` manifest creates a dedicated `kong-migrator` namespace and installs all required resources (ServiceAccount, ClusterRole, ClusterRoleBinding, Deployment, Service):

```bash
kubectl apply -f deploy/kubernetes.yaml
```

Wait for the pod to become ready:

```bash
kubectl -n kong-migrator rollout status deployment/kong-migrator
```

#### Access the Dashboard

Port-forward the service to your local machine:

```bash
kubectl -n kong-migrator port-forward svc/kong-migrator 8080:8080
```

Then open [http://localhost:8080](http://localhost:8080) in your browser. The dashboard reads Ingress resources across all namespaces using the pod's service account — no kubeconfig or extra credentials required.

To restrict the dashboard to a single namespace, edit the Deployment and add `--namespace <your-namespace>` to the `args` list.

### Uninstall

```bash
kubectl delete -f deploy/kubernetes.yaml
```

This removes all resources created by the manifest, including the `kong-migrator` namespace.

## Supported Annotations

The tool supports **25+ NGINX Ingress annotations** across multiple categories:

### Routing & Rewrites
| NGINX Annotation | Kong Output | Notes |
|------------------|-------------|-------|
| `rewrite-target` | Kong Ingress: `konghq.com/rewrite`<br>Gateway API: `HTTPRoute.filters.urlRewrite` | Static rewrites supported |
| `backend-protocol` | `konghq.com/protocol` | HTTP, HTTPS, GRPC |
| `ssl-redirect` | `konghq.com/https-redirect-status-code` (301) | Force HTTPS |
| `force-ssl-redirect` | `konghq.com/https-redirect-status-code` (301) | Force HTTPS |
| `app-root` | `konghq.com/rewrite` | Redirect `/` to specified path |

### Rate Limiting & Traffic Control
| NGINX Annotation | Kong Output | Notes |
|------------------|-------------|-------|
| `limit-rps` | `rate-limiting` Plugin | Requests per second |
| `limit-rpm` | `rate-limiting` Plugin | Requests per minute |
| `limit-connections` | Warning + Manual Config | Requires Enterprise `rate-limiting-advanced` |

### Authentication & Security
| NGINX Annotation | Kong Output | Notes |
|------------------|-------------|-------|
| `auth-url` | `external-auth` Plugin | External authentication service |
| `auth-signin` | `openid-connect` Plugin | **Enterprise** - OIDC with placeholders |
| `enable-cors` | Kong Ingress: `cors` Plugin<br>Gateway API: `ResponseHeaderModifier` | CORS headers |
| `cors-allow-origin` | `cors` Plugin | Allowed origins |
| `cors-allow-methods` | `cors` Plugin | Allowed HTTP methods |
| `cors-allow-headers` | `cors` Plugin | Allowed headers |
| `cors-allow-credentials` | `cors` Plugin | Allow credentials |
| `cors-max-age` | `cors` Plugin | Preflight cache duration |
| `whitelist-source-range` | `ip-restriction` Plugin | IP allowlist |

### Request/Response Handling
| NGINX Annotation | Kong Output | Notes |
|------------------|-------------|-------|
| `proxy-body-size` | `request-size-limiting` Plugin | Max request body size |
| `configuration-snippet` (headers) | `request-transformer` & `response-transformer` Plugins | Header injection only |
| `server-snippet` | Warning | Not supported |

### Timeouts
| NGINX Annotation | Kong Output | Notes |
|------------------|-------------|-------|
| `proxy-connect-timeout` | `konghq.com/connect-timeout` | Connect timeout in ms |
| `proxy-read-timeout` | `konghq.com/read-timeout` | Read timeout in ms | 
| `proxy-send-timeout` | `konghq.com/write-timeout` | Write timeout in ms |

### Advanced Features
| NGINX Annotation | Kong Output | Notes |
|------------------|-------------|-------|
| `affinity` | `KongUpstreamPolicy` | Cookie-based session affinity |
| `session-cookie-name` | `KongUpstreamPolicy` | Custom cookie name for affinity |
| `session-cookie-max-age` | `KongUpstreamPolicy` | Cookie TTL |
| `canary` | Kong Ingress: `canary` Plugin<br>Gateway API: Weighted `backendRefs` | Canary deployments |
| `canary-weight` | Kong Ingress: `canary` Plugin<br>Gateway API: Weighted `backendRefs` | Traffic splitting percentage |
| `upstream-keepalive-connections` | Warning | Global Kong setting, not per-upstream |

### Caching (Enterprise)
| NGINX Annotation | Kong Output | Notes |
|------------------|-------------|-------|
| `proxy-cache` | `proxy-cache` Plugin | **Enterprise** - Response caching |
| `proxy-cache-valid` | `proxy-cache` Plugin | Cache TTL configuration |

### Traffic Mirroring
| NGINX Annotation | Kong Output | Notes |
|------------------|-------------|-------|
| `mirror-target` | Warning + Manual Config | Requires custom plugin (not bundled) |

### Not Supported (Logged for Feedback)
The following annotations are detected but not automatically migrated:
- `auth-method` - No direct equivalent
- `auth-response-headers` - No direct equivalent  
- `base-auth-secret` - Use Kong's basic-auth plugin manually
- `preserve-trailing-slash` - Kong default behavior
- `configuration-snippet` (non-header) - Complex snippets not supported
- `modsecurity-*` - Third-party WAF integration required

## CLI Usage

### Build
```bash
go build -o migrator main.go
```

### Run Migration

#### Kong Ingress Format (Default)
```bash
./migrator --input input.yaml --output output.yaml
```

#### Gateway API Format
```bash
./migrator --input input.yaml --output httproute.yaml --output-format gateway-api
```

#### Both Formats
```bash
./migrator --input input.yaml --output-format both
# Generates: migrated-kong-ingress.yaml & migrated-gateway-api.yaml
```

**Flags**:
- `--input`: Path to input YAML containing NGINX Ingress resources
- `--output`: Path to output YAML (default: `migrated-output.yaml`)
- `--output-format`: Output format - `kong-ingress` (default), `gateway-api`, or `both`
- `--ingress-class`: (Optional) Set the `ingressClassName` in the generated Ingress

### Apply to Kubernetes

**Kong Ingress:**
```bash
kubectl apply -f migrated-kong-ingress.yaml
```

**Gateway API:**
```bash
# Install Gateway API CRDs (one-time)
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml

# Deploy HTTPRoute and plugins
kubectl apply -f migrated-gateway-api.yaml
```

*Note: Some features like `affinity` generate `KongUpstreamPolicy` resources which require you to MANUALLY annotate your Service. The tool will log these actions.*

## Gateway API Support

The tool supports migrating to Kubernetes Gateway API (HTTPRoute) format, offering:

**Native Gateway API Features**:
- **URL Rewrites**: `rewrite-target` → `HTTPRoute.filters.urlRewrite`
- **Canary Routing**: `canary-weight` → weighted `backendRefs`
- **CORS Headers**: `enable-cors` → `ResponseHeaderModifier` filter

**Kong Plugin Integration** (via ExtensionRef):
- Rate limiting, authentication, caching, and other advanced features
- Plugins attached to HTTPRoute using `ExtensionRef` filters
- Full compatibility with Kong-specific features

**Benefits**:
- Standards-based (works across implementations)
- Portable configurations
- Future-proof with Kubernetes Gateway API

### Example Output

**Input (NGINX Ingress)**:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api
  annotations:
    nginx.ingress.kubernetes.io/enable-cors: "true"
    nginx.ingress.kubernetes.io/canary-weight: "20"
spec:
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /v1
        backend:
          service:
            name: api-service
            port:
              number: 80
```

**Output (Gateway API)**:
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api
spec:
  hostnames:
  - api.example.com
  parentRefs:
  - name: kong
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /v1
    filters:
    - type: ResponseHeaderModifier
      responseHeaderModifier:
        add:
        - name: Access-Control-Allow-Origin
          value: "*"
    backendRefs:
    - name: api-service
      port: 80
      weight: 80
    - name: api-service-canary
      port: 80
      weight: 20
```


---

## End-to-End Testing

The project includes a robust E2E test script `e2e-test.sh` that works with Orbstack, Minikube, or Kind.

### Prerequisites
- Docker / Orbstack
- Helm
- Go 1.22+
- `kubectl` configured for your cluster

### Running the Test
```bash
./e2e-test.sh
```

**Enterprise Testing**:
To enable Kong Enterprise features (like `openid-connect`) in the test:
1. Place your `license.json` file in the root of this project.
2. Run `./e2e-test.sh`. The script will automatically detect the license and deploy Kong Gateway Enterprise.

This script will:
1. Install NGINX and Kong Controllers via Helm.
2. Deploy a full-coverage backend application (`ealen/echo-server`).
3. Run the migration tool against a complex "kitchen sink" ingress.
4. Apply the result and wait for Kong to program routes.
5. **Verify Traffic**: Sends real HTTP requests to validate Connectivity, CORS, Headers, Rate Limiting, and Session Affinity.

## Architecture

The tool is written in Go and structured as follows:
- `main.go`: Entry point, parses flags and input files.
- `pkg/parser`: Reads Kubernetes YAML manifests.
- `pkg/mappers`: Contains logic for each annotation category (e.g., `ratelimit.go`, `auth.go`).
- `pkg/generator`: specific structs and logic for generating Kong CRDs (`KongPlugin`, `KongUpstreamPolicy`).

## Limitations

- **Regex Rewrites**: Complex NGINX regex captures in `rewrite-target` are not fully automated and may require manual `request-transformer` configuration.
- **Snippets**: Arbitrary NGINX configuration snippets are not supported, except for specific header directives which are parsed into transformers.
