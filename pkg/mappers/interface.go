package mappers

import (
	"fmt"
	"strings"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// Apply applies all known mappings to an Ingress object.
func Apply(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin, kongIngresses *[]generator.KongIngress, upstreamPolicies *[]generator.KongUpstreamPolicy) {
	// Helper to ensure clean conversion
	if ing.Annotations == nil {
		ing.Annotations = make(map[string]string)
	}

	// Routing Mappings
	mapRewrite(ing)
	mapProtocol(ing)
	mapSSLRedirect(ing)
	MapServiceUpstream(ing)
	MapUseRegex(ing)
	MapPriority(ing)

	// Traffic Mappings
	mapRateLimit(ing, plugins)
	mapAuth(ing, plugins)
	mapSecurity(ing, plugins)
	mapTimeouts(ing, plugins)
	mapCanary(ing, plugins)
	mapRequestHandling(ing, plugins)
	MapClientMaxBodySize(ing, plugins)

	// Security: mTLS client certificate authentication
	MapMTLS(ing, plugins)

	// Request Termination: Maintenance mode and custom error pages
	MapMaintenanceMode(ing, plugins)
	MapDefaultBackendResponse(ing, plugins)

	// Redirects: Permanent and temporal redirects
	MapPermanentRedirect(ing, plugins)
	MapTemporalRedirect(ing, plugins)

	// Bot Detection: User-Agent filtering
	MapBotDetection(ing, plugins)

	// Apply other mappings...
	// Note: We are now passing explicit pointers to everything.
	// mapAdvanced handles: affinity, limit-connections, mirror, upstream-keepalive

	// Ensure we don't call legacy mapAffinity or mapUpstreamKeepalive separately if mapAdvanced covers them.
	// The `mapAdvanced` implementation calls them. So we just call `mapAdvanced`.

	// Batch 8: External Auth & Headers
	mapExternalAuth(ing, plugins)
	// mapSnippetHeaders is likely called inside or concurrently?
	// The previous implementation had them separate. Let's keep them separate if not in mapAdvanced.
	mapSnippetHeaders(ing, plugins)
	// Batch 9: Advanced (Affinity, etc.)
	mapAdvanced(ing, plugins, kongIngresses, upstreamPolicies)

	// Batch 10: Caching
	mapCaching(ing, plugins)

	// Phase 4: Keepalive warnings
	MapUpstreamKeepaliveTimeout(ing)
	MapUpstreamKeepaliveRequests(ing)

	// Phase 5: Unsupported annotations (warnings)
	WarnBufferSettings(ing)
	WarnSnippets(ing)
	WarnDefaultBackend(ing)
	WarnHTTP2PushPreload(ing)
	WarnProxyHTTPVersion(ing)
}

// removeAnnotation removes an annotation from the Ingress
func removeAnnotation(ing *networkingv1.Ingress, key string) {
	delete(ing.Annotations, key)
}

// addAnnotation adds an annotation to the Ingress
func addAnnotation(ing *networkingv1.Ingress, key, value string) {
	if ing.Annotations == nil {
		ing.Annotations = make(map[string]string)
	}
	ing.Annotations[key] = value
}

// addPluginReference adds a plugin to the konghq.com/plugins annotation
func addPluginReference(ing *networkingv1.Ingress, pluginName string) {
	const key = "konghq.com/plugins"
	current := ing.Annotations[key]
	if current == "" {
		addAnnotation(ing, key, pluginName)
	} else {
		// Avoid duplicates (checking simplisticly)
		if !strings.Contains(current, pluginName) {
			addAnnotation(ing, key, current+", "+pluginName)
		}
	}
}

// generateName helper to create a unique plugin name
func generateName(ingName, pluginType string) string {
	return fmt.Sprintf("%s-%s", ingName, pluginType)
}
