package mappers

import (
	"log"
	"strings"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// mapAdvanced handles complex annotations like affinity, mirror, etc.
func mapAdvanced(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin, kongIngresses *[]generator.KongIngress, upstreamPolicies *[]generator.KongUpstreamPolicy) {
	MapAffinity(ing, plugins, kongIngresses, upstreamPolicies) // Updated function call
	mapLimitConnections(ing)
	mapMirror(ing, plugins)
	mapUpstreamKeepalive(ing, upstreamPolicies)
}

func MapAffinity(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin, kongIngresses *[]generator.KongIngress, upstreamPolicies *[]generator.KongUpstreamPolicy) {
	key := "nginx.ingress.kubernetes.io/affinity"
	cookieNameKey := "nginx.ingress.kubernetes.io/session-cookie-name"
	cookieExpiresKey := "nginx.ingress.kubernetes.io/session-cookie-expires"
	cookieHashKey := "nginx.ingress.kubernetes.io/session-cookie-hash"
	cookieMaxAgeKey := "nginx.ingress.kubernetes.io/session-cookie-max-age"

	affinity, ok := ing.Annotations[key]
	if !ok || strings.TrimSpace(affinity) != "cookie" { // Changed val to affinity and added TrimSpace
		// Clean up any orphaned session-cookie annotations
		removeAnnotation(ing, cookieNameKey)
		removeAnnotation(ing, cookieExpiresKey)
		removeAnnotation(ing, cookieHashKey)
		removeAnnotation(ing, cookieMaxAgeKey)
		return
	}

	// Build KongUpstreamPolicy for cookie-based affinity
	policyName := generateName(ing.Name, "upstream-affinity")

	// Session cookie name
	cookieName := "route" // Default
	if c, ok := ing.Annotations[cookieNameKey]; ok {
		cookieName = c
		removeAnnotation(ing, cookieNameKey)
	}

	policy := generator.KongUpstreamPolicy{
		APIVersion: "configuration.konghq.com/v1beta1",
		Kind:       "KongUpstreamPolicy",
		Metadata: generator.ObjectMeta{
			Name:      policyName,
			Namespace: ing.Namespace,
		},
		Spec: map[string]interface{}{
			"hashOn": map[string]interface{}{
				"cookie":     cookieName,
				"cookiePath": "/",
			},
			"algorithm": "consistent-hashing",
		},
	}

	// Session cookie TTL (from max-age or expires)
	// Both map to the same Kong field - prefer max-age if both present
	if maxAge, ok := ing.Annotations[cookieMaxAgeKey]; ok {
		log.Printf("INFO: Ingress %s/%s specifies session-cookie-max-age=%s. Kong's KongUpstreamPolicy uses cookie-based hashing without explicit TTL control. Consider using Set-Cookie headers for TTL management.", ing.Namespace, ing.Name, maxAge)
		removeAnnotation(ing, cookieMaxAgeKey)
	} else if expires, ok := ing.Annotations[cookieExpiresKey]; ok {
		log.Printf("INFO: Ingress %s/%s specifies session-cookie-expires=%s. Kong's KongUpstreamPolicy uses cookie-based hashing. TTL management should be handled via Set-Cookie headers.", ing.Namespace, ing.Name, expires)
		removeAnnotation(ing, cookieExpiresKey)
	}

	// Cookie hash algorithm
	// NGINX uses this for hashing the cookie value, Kong has different hash mechanisms
	if hash, ok := ing.Annotations[cookieHashKey]; ok {
		log.Printf("INFO: Ingress %s/%s specifies session-cookie-hash=%s. Kong handles cookie-based load balancing differently. The hash algorithm is managed internally.", ing.Namespace, ing.Name, hash)
		removeAnnotation(ing, cookieHashKey)
	}

	*upstreamPolicies = append(*upstreamPolicies, policy)

	// Annotate Service
	log.Printf("ACTION REQUIRED: Ingress %s/%s uses 'affinity: cookie'. Generated KongUpstreamPolicy '%s'. You must MANUALLY annotate your Service(s) with: 'konghq.com/upstream-policy: %s'", ing.Namespace, ing.Name, policyName, policyName)

	removeAnnotation(ing, key)
}

func mapLimitConnections(ing *networkingv1.Ingress) {
	key := "nginx.ingress.kubernetes.io/limit-connections"
	if val, ok := ing.Annotations[key]; ok {
		log.Printf("ACTION REQUIRED: Ingress %s/%s uses 'limit-connections: %s'. Kong Open Source does not support concurrent connection limits. For Enterprise, configure the 'rate-limiting-advanced' plugin with 'limit: [%s]' and 'window_type: sliding'.", ing.Namespace, ing.Name, val, val)
		removeAnnotation(ing, key)
	}
}

func mapMirror(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	// NGINX mirror-target is often a backend service name or URI.
	// Kong does not have a bundled 'proxy-mirror' plugin.
	// This functionality requires a custom plugin (e.g. kong-plugin-http-mirror) or Enterprise customization.

	key := "nginx.ingress.kubernetes.io/mirror-target"
	if val, ok := ing.Annotations[key]; ok {
		log.Printf("ACTION REQUIRED: Ingress %s/%s uses 'mirror-target: %s'. Kong does not have a bundled mirroring plugin. You must install a custom plugin (e.g. 'kong-plugin-http-mirror') and configure it manually.", ing.Namespace, ing.Name, val)
		removeAnnotation(ing, key)
	}
}

func mapUpstreamKeepalive(ing *networkingv1.Ingress, upstreamPolicies *[]generator.KongUpstreamPolicy) {
	key := "nginx.ingress.kubernetes.io/upstream-keepalive-connections"
	if _, ok := ing.Annotations[key]; ok {
		log.Printf("WARNING: Ingress %s/%s uses '%s'. KongUpstreamPolicy/KongIngress does not currently expose granular per-upstream keepalive pool size configuration in KIC. This is usually a global setting in Kong configuration.", ing.Namespace, ing.Name, key)
		removeAnnotation(ing, key)
	}
}

// MapUpstreamKeepaliveTimeout handles upstream-keepalive-timeout annotation
func MapUpstreamKeepaliveTimeout(ing *networkingv1.Ingress) {
	key := "nginx.ingress.kubernetes.io/upstream-keepalive-timeout"

	if _, ok := ing.Annotations[key]; ok {
		log.Printf("WARNING: Ingress %s/%s uses '%s'. Kong manages upstream keepalive globally via 'upstream_keepalive_idle_timeout' configuration. This cannot be set per-Ingress in KIC.", ing.Namespace, ing.Name, key)
		removeAnnotation(ing, key)
	}
}

// MapUpstreamKeepaliveRequests handles upstream-keepalive-requests annotation
func MapUpstreamKeepaliveRequests(ing *networkingv1.Ingress) {
	key := "nginx.ingress.kubernetes.io/upstream-keepalive-requests"

	if _, ok := ing.Annotations[key]; ok {
		log.Printf("WARNING: Ingress %s/%s uses '%s'. Kong manages upstream keepalive globally via 'upstream_keepalive_max_requests' configuration. This cannot be set per-Ingress in KIC.", ing.Namespace, ing.Name, key)
		removeAnnotation(ing, key)
	}
}

// WarnBufferSettings handles NGINX-specific buffer annotations that have no Kong equivalent
func WarnBufferSettings(ing *networkingv1.Ingress) {
	bufferAnnotations := map[string]string{
		"nginx.ingress.kubernetes.io/client-header-buffer-size":   "NGINX client header buffer configuration",
		"nginx.ingress.kubernetes.io/client-body-buffer-size":     "NGINX client body buffer configuration",
		"nginx.ingress.kubernetes.io/proxy-buffer-size":           "NGINX proxy buffer configuration",
		"nginx.ingress.kubernetes.io/proxy-buffers-number":        "NGINX proxy buffers count configuration",
		"nginx.ingress.kubernetes.io/large-client-header-buffers": "NGINX large header buffer configuration",
		"nginx.ingress.kubernetes.io/http2-max-header-size":       "HTTP/2 max header size configuration",
	}

	for key, description := range bufferAnnotations {
		if _, ok := ing.Annotations[key]; ok {
			log.Printf("INFO: Ingress %s/%s uses '%s' (%s). Kong does not expose NGINX-level buffer tuning. These are global Kong/NGINX settings configured at the gateway level, not per-Ingress.", ing.Namespace, ing.Name, key, description)
			removeAnnotation(ing, key)
		}
	}
}

// WarnSnippets handles snippet annotations that cannot be migrated
func WarnSnippets(ing *networkingv1.Ingress) {
	snippets := map[string]string{
		"nginx.ingress.kubernetes.io/server-snippet": "arbitrary NGINX server block configuration",
		"nginx.ingress.kubernetes.io/auth-snippet":   "arbitrary NGINX auth configuration",
	}

	for key, description := range snippets {
		if _, ok := ing.Annotations[key]; ok {
			log.Printf("WARNING: Ingress %s/%s uses '%s' (%s). Arbitrary NGINX snippets are not supported in Kong. Consider using Kong plugins for equivalent functionality.", ing.Namespace, ing.Name, key, description)
			removeAnnotation(ing, key)
		}
	}
}

// WarnDefaultBackend handles default-backend annotation
func WarnDefaultBackend(ing *networkingv1.Ingress) {
	key := "nginx.ingress.kubernetes.io/default-backend"

	if _, ok := ing.Annotations[key]; ok {
		log.Printf("INFO: Ingress %s/%s uses '%s'. This is typically configured in the Ingress spec's defaultBackend field, not as an annotation. Kong supports default backends via the Ingress spec.", ing.Namespace, ing.Name, key)
		removeAnnotation(ing, key)
	}
}

// WarnHTTP2PushPreload handles http2-push-preload annotation
func WarnHTTP2PushPreload(ing *networkingv1.Ingress) {
	key := "nginx.ingress.kubernetes.io/http2-push-preload"

	if _, ok := ing.Annotations[key]; ok {
		log.Printf("INFO: Ingress %s/%s uses 'http2-push-preload'. Kong supports HTTP/2 but does not have automatic server push based on Link headers. HTTP/2 server push can be implemented manually via response headers or custom plugins.", ing.Namespace, ing.Name)
		removeAnnotation(ing, key)
	}
}

// WarnProxyHTTPVersion handles proxy-http-version annotation
func WarnProxyHTTPVersion(ing *networkingv1.Ingress) {
	key := "nginx.ingress.kubernetes.io/proxy-http-version"

	if val, ok := ing.Annotations[key]; ok {
		log.Printf("INFO: Ingress %s/%s specifies proxy-http-version='%s'. Kong's upstream HTTP version is controlled globally via 'upstream_http_version' configuration (default: 1.1). This cannot be set per-Ingress.", ing.Namespace, ing.Name, val)
		removeAnnotation(ing, key)
	}
}

// ptr is a helper function to return a pointer to a value
func ptr(s string) *string {
	return &s
}
