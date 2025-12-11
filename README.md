# NGINX to Kong Ingress Migration Tool

This tool automates the migration of Kubernetes Ingress resources from **NGINX Ingress Controller** to **Kong Ingress Controller (KIC)**. It parses existing NGINX annotations and translates them into Kong-equivalent configurations, generating `Ingress`, `KongPlugin`, and `KongUpstreamPolicy` resources.

## Features

- **Automated Translation**: Converts NGINX annotations to Kong Plugins and CRDs.
- **Modern Kong Support**: targeting `KongUpstreamPolicy` for upstream configurations (replacing deprecated `KongIngress`).
- **Full Coverage Testing**: Includes an End-to-End (E2E) test suite validating the migration against a live cluster.
- **Safe Defaults**: Generates standard configuration while warning about incompatible or manual-intervention items.

## Supported Annotations

The tool currently supports the following categories of annotations:

| Category | NGINX Annotation | Kong Equivalent |
|----------|------------------|-----------------|
| **Routing** | `rewrite-target` | `konghq.com/rewrite` (Static) |
| | `backend-protocol` | `konghq.com/protocol` |
| | `ssl-redirect` / `force-ssl-redirect` | `konghq.com/https-redirect-status-code` |
| **Traffic** | `limit-rps`, `limit-rpm` | `rate-limiting` Plugin |
| **Auth** | `auth-url`, `auth-signin` | `external-auth` Plugin (Note: requires manual IDP tuning) |
| **Security** | `enable-cors`, `cors-allow-*` | `cors` Plugin |
| | `whitelist-source-range` | `ip-restriction` Plugin |
| | `enable-modsecurity` | No direct match. *Placeholder generated for third-party WAF (e.g. open-appsec, Wallarm)* |
| **Auth** | `auth-url`, `auth-signin` | `external-auth` or `openid-connect` (Enterprise) - *Generated with placeholders* |
| **Caching** | `proxy-cache`, `proxy-cache-valid` | `proxy-cache` Plugin |
| **Traffic** | `mirror-target` | `proxy-mirror` Plugin |
| **Timeouts** | `proxy-connect-timeout`, `proxy-read-timeout`, `proxy-send-timeout` | `konghq.com/connect-timeout` etc. |
| **Canary** | `canary`, `canary-weight` | `canary` Plugin |
| **Headers** | `proxy-body-size` | `konghq.com/request-size` (via Plugin) |
| | `configuration-snippet` (headers) | `request-transformer` Plugin |
| **Advanced** | `affinity` | `KongUpstreamPolicy` (hashOn: cookie) |

## Usage

### Build
```bash
go build -o migrator main.go
```

### Run Migration
```bash
./migrator -f input.yaml -o output.yaml -ingress-class kong
```

- `-f`: Path to input YAML containing NGINX Ingress resources.
- `-o`: Path to output YAML for Kong resources.
- `-ingress-class`: (Optional) Set the `ingressClassName` in the generated Ingress.

### Apply to Kubernetes
```bash
kubectl apply -f output.yaml
```
*Note: Some features like `affinity` generate `KongUpstreamPolicy` resources which require you to MANUALLY annotate your Service. The tool will log these actions.*

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
