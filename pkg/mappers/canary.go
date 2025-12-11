package mappers

import (
	"strconv"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// mapCanary handles canary annotations
// NGINX: nginx.ingress.kubernetes.io/canary: "true"
// NGINX: nginx.ingress.kubernetes.io/canary-weight: "50"
// KONG: KongPlugin (canary) - Note: This is usually an Enterprise plugin or requires logic.
func mapCanary(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	key := "nginx.ingress.kubernetes.io/canary"
	if val, ok := ing.Annotations[key]; !ok || val != "true" {
		return
	}

	config := make(map[string]interface{})

	// Map Weight
	weightKey := "nginx.ingress.kubernetes.io/canary-weight"
	if val, ok := ing.Annotations[weightKey]; ok {
		if i, err := strconv.Atoi(val); err == nil {
			config["percentage"] = i
		}
		removeAnnotation(ing, weightKey)
	}

	// Map Header
	headerKey := "nginx.ingress.kubernetes.io/canary-by-header"
	if val, ok := ing.Annotations[headerKey]; ok {
		config["hash"] = "header"
		config["hash_header"] = val
		removeAnnotation(ing, headerKey)
	}

	// Default hash if just weight
	if _, ok := config["hash"]; !ok {
		// NGINX default is cookie? Kong default is consumer?
		// Let's set to "consumer" or "ip" generic
		// For now, leaving empty to let plugin default decide or just 'percentage'
	}

	pluginName := generateName(ing.Name, "canary")
	plugin := generator.KongPlugin{
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "canary",
		Config: config,
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)
	removeAnnotation(ing, key)
}
