package mappers

import (
	"strings"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// mapSecurity handles security-related annotations
// NGINX: whitelist-source-range -> Kong: ip-restriction (allow)
// NGINX: enable-cors, cors-* -> Kong: cors
func mapSecurity(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	mapIPRestriction(ing, plugins)
	mapIPRestriction(ing, plugins)
	mapCORS(ing, plugins)
}

func mapIPRestriction(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	key := "nginx.ingress.kubernetes.io/whitelist-source-range"
	if val, ok := ing.Annotations[key]; ok {
		// NGINX: comma separated CIDRs
		// Kong: config.allow list of strings

		cidrs := strings.Split(val, ",")
		for i := range cidrs {
			cidrs[i] = strings.TrimSpace(cidrs[i])
		}

		pluginName := generateName(ing.Name, "ip-restriction")
		plugin := generator.KongPlugin{
			Metadata: generator.ObjectMeta{
				Name:      pluginName,
				Namespace: ing.Namespace,
			},
			Plugin: "ip-restriction",
			Config: map[string]interface{}{
				"allow": cidrs,
			},
		}

		*plugins = append(*plugins, plugin)
		addPluginReference(ing, pluginName)
		removeAnnotation(ing, key)
	}
}

func mapCORS(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	enableKey := "nginx.ingress.kubernetes.io/enable-cors"
	if val, ok := ing.Annotations[enableKey]; !ok || val != "true" {
		return
	}

	config := make(map[string]interface{})

	// Map CORS sub-annotations
	// Origin
	if val, ok := ing.Annotations["nginx.ingress.kubernetes.io/cors-allow-origin"]; ok {
		// NGINX allows single string or regex. Kong expects list.
		// If specific origin, put in list. Use '*' handling carefully.
		if val == "*" {
			// Kong defaults typically handle * or we leave it empty/default depending on version users want
			// logic: Kong 'origins' can be set to ['*']
		}
		config["origins"] = []string{val}
		removeAnnotation(ing, "nginx.ingress.kubernetes.io/cors-allow-origin")
	}

	// Methods
	if val, ok := ing.Annotations["nginx.ingress.kubernetes.io/cors-allow-methods"]; ok {
		methods := strings.Split(val, ",")
		for i := range methods {
			methods[i] = strings.TrimSpace(methods[i])
		}
		config["methods"] = methods
		removeAnnotation(ing, "nginx.ingress.kubernetes.io/cors-allow-methods")
	}

	// Headers
	if val, ok := ing.Annotations["nginx.ingress.kubernetes.io/cors-allow-headers"]; ok {
		headers := strings.Split(val, ",")
		for i := range headers {
			headers[i] = strings.TrimSpace(headers[i])
		}
		config["headers"] = headers
		removeAnnotation(ing, "nginx.ingress.kubernetes.io/cors-allow-headers")
	}

	// Credentials
	if val, ok := ing.Annotations["nginx.ingress.kubernetes.io/cors-allow-credentials"]; ok {
		if val == "true" {
			config["credentials"] = true
		}
		removeAnnotation(ing, "nginx.ingress.kubernetes.io/cors-allow-credentials")
	}

	pluginName := generateName(ing.Name, "cors")
	plugin := generator.KongPlugin{
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "cors",
		Config: config,
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)
	removeAnnotation(ing, enableKey)
}
