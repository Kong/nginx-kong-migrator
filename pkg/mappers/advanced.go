package mappers

import (
	"log"
	"strings"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// mapAdvanced handles complex annotations like affinity, mirror, etc.
func mapAdvanced(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin, kongIngresses *[]generator.KongIngress, upstreamPolicies *[]generator.KongUpstreamPolicy) {
	mapAffinity(ing, upstreamPolicies)
	mapLimitConnections(ing)
	mapMirror(ing, plugins)
	mapUpstreamKeepalive(ing, upstreamPolicies)
}

func mapAffinity(ing *networkingv1.Ingress, upstreamPolicies *[]generator.KongUpstreamPolicy) {
	key := "nginx.ingress.kubernetes.io/affinity"
	cookieNameKey := "nginx.ingress.kubernetes.io/session-cookie-name"

	val, ok := ing.Annotations[key]
	if !ok || strings.TrimSpace(val) != "cookie" {
		return
	}

	cookieName := "route" // Default
	if c, ok := ing.Annotations[cookieNameKey]; ok {
		cookieName = c
		removeAnnotation(ing, cookieNameKey)
	}

	// Create KongUpstreamPolicy
	name := generateName(ing.Name, "upstream-affinity")

	policy := generator.KongUpstreamPolicy{
		APIVersion: "configuration.konghq.com/v1beta1",
		Kind:       "KongUpstreamPolicy",
		Metadata: generator.ObjectMeta{
			Name:      name,
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

	*upstreamPolicies = append(*upstreamPolicies, policy)

	// Annotate Service
	log.Printf("ACTION REQUIRED: Ingress %s/%s uses 'affinity: cookie'. Generated KongUpstreamPolicy '%s'. You must MANUALLY annotate your Service(s) with: 'konghq.com/upstream-policy: %s'", ing.Namespace, ing.Name, name, name)

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
