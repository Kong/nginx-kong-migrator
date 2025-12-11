package mappers

import (
	"strconv"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// mapRateLimit handles rate limiting annotations
// NGINX: nginx.ingress.kubernetes.io/limit-rps
// NGINX: nginx.ingress.kubernetes.io/limit-rpm
// NGINX: nginx.ingress.kubernetes.io/limit-connections
// KONG: KongPlugin (rate-limiting)
func mapRateLimit(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	rpsKey := "nginx.ingress.kubernetes.io/limit-rps"
	rpmKey := "nginx.ingress.kubernetes.io/limit-rpm"
	// connKey := "nginx.ingress.kubernetes.io/limit-connections" // Kong Ent or advanced config usually

	rpsVal, hasRPS := ing.Annotations[rpsKey]
	rpmVal, hasRPM := ing.Annotations[rpmKey]

	if !hasRPS && !hasRPM {
		return
	}

	config := make(map[string]interface{})

	if hasRPS {
		val, _ := strconv.Atoi(rpsVal)
		config["second"] = val
		removeAnnotation(ing, rpsKey)
	}

	if hasRPM {
		val, _ := strconv.Atoi(rpmVal)
		config["minute"] = val
		removeAnnotation(ing, rpmKey)
	}

	// Create Plugin
	pluginName := generateName(ing.Name, "rate-limiting")
	plugin := generator.KongPlugin{
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "rate-limiting",
		Config: config,
	}

	*plugins = append(*plugins, plugin)

	// Link Plugin
	addPluginReference(ing, pluginName)
}
