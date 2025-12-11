package mappers

import (
	networkingv1 "k8s.io/api/networking/v1"
)

// mapRewrite handles nginx.ingress.kubernetes.io/rewrite-target
// NGINX: rewrite-target: /$1 (regex capture) OR /foo (static)
// KONG: konghq.com/rewrite: /foo (static) OR Request Transformer (regex)
//
// For this MVP, we map simple static rewrites or direct 1:1 mappings.
// Complex regex capture groups ($1, $2) often require Request Transformer in Kong.
func mapRewrite(ing *networkingv1.Ingress) {
	const nginxKey = "nginx.ingress.kubernetes.io/rewrite-target"
	const kongKey = "konghq.com/rewrite"

	if val, ok := ing.Annotations[nginxKey]; ok {
		// NGINX often uses /$1 or target string.
		// Kong's simple rewrite annotation supports static strings.
		// If NGINX config is `/target/$1`, we should just migrate `/target` for the simple annotation
		// OR we need to use a plugin.
		// Let's implement the simpler `konghq.com/rewrite` mapping first for direct path rewrites.

		// Heuristic: if it contains $, it's a regex capture.
		// Kong's `strip-path` + `rewrite` combination often handles this differently.

		addAnnotation(ing, kongKey, val)
		removeAnnotation(ing, nginxKey)
	}
}

// mapProtocol handles nginx.ingress.kubernetes.io/backend-protocol
// Values: HTTP, HTTPS, GRPC, GRPCS
// Kong: konghq.com/protocol
func mapProtocol(ing *networkingv1.Ingress) {
	const nginxKey = "nginx.ingress.kubernetes.io/backend-protocol"
	const kongKey = "konghq.com/protocol"

	if val, ok := ing.Annotations[nginxKey]; ok {
		addAnnotation(ing, kongKey, val) // Values match (http, https, grpc)
		removeAnnotation(ing, nginxKey)
	}
}

// mapSSLRedirect handles nginx.ingress.kubernetes.io/ssl-redirect and force-ssl-redirect
func mapSSLRedirect(ing *networkingv1.Ingress) {
	// NGINX: ssl-redirect: "true" (default) or "false"
	// KONG: konghq.com/https-redirect-status-code: "301" (to enable)

	key := "nginx.ingress.kubernetes.io/force-ssl-redirect"
	val, ok := ing.Annotations[key]
	if !ok {
		key = "nginx.ingress.kubernetes.io/ssl-redirect"
		val, ok = ing.Annotations[key]
	}

	if ok && val == "true" {
		addAnnotation(ing, "konghq.com/https-redirect-status-code", "301")
	}

	// Clean up both if present
	removeAnnotation(ing, "nginx.ingress.kubernetes.io/force-ssl-redirect")
	removeAnnotation(ing, "nginx.ingress.kubernetes.io/ssl-redirect")
}
