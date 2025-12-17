package mappers

import (
	"log"
	"strings"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// MapPermanentRedirect handles permanent redirect annotation
// NGINX: nginx.ingress.kubernetes.io/permanent-redirect
// Kong: redirect plugin with status 301
func MapPermanentRedirect(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	key := "nginx.ingress.kubernetes.io/permanent-redirect"

	location, ok := ing.Annotations[key]
	if !ok || strings.TrimSpace(location) == "" {
		return
	}

	pluginName := generateName(ing.Name, "permanent-redirect")

	plugin := generator.KongPlugin{
		APIVersion: "configuration.konghq.com/v1",
		Kind:       "KongPlugin",
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "redirect",
		Config: map[string]interface{}{
			"status_code": 301,
			"location":    location,
		},
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)

	log.Printf("INFO: Ingress %s/%s configured permanent redirect (301) to %s", ing.Namespace, ing.Name, location)
	removeAnnotation(ing, key)
}

// MapTemporalRedirect handles temporal/temporary redirect annotation
// NGINX: nginx.ingress.kubernetes.io/temporal-redirect
// Kong: redirect plugin with status 302
func MapTemporalRedirect(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	key := "nginx.ingress.kubernetes.io/temporal-redirect"

	location, ok := ing.Annotations[key]
	if !ok || strings.TrimSpace(location) == "" {
		return
	}

	pluginName := generateName(ing.Name, "temporal-redirect")

	plugin := generator.KongPlugin{
		APIVersion: "configuration.konghq.com/v1",
		Kind:       "KongPlugin",
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "redirect",
		Config: map[string]interface{}{
			"status_code": 302,
			"location":    location,
		},
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)

	log.Printf("INFO: Ingress %s/%s configured temporal redirect (302) to %s", ing.Namespace, ing.Name, location)
	removeAnnotation(ing, key)
}

// MapBotDetection handles bot detection annotation
// NGINX: No standard annotation (custom)
// Kong: bot-detection plugin
func MapBotDetection(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	allowKey := "nginx.ingress.kubernetes.io/bot-detection-allow"
	denyKey := "nginx.ingress.kubernetes.io/bot-detection-deny"

	allowList, hasAllow := ing.Annotations[allowKey]
	denyList, hasDeny := ing.Annotations[denyKey]

	if !hasAllow && !hasDeny {
		return
	}

	pluginName := generateName(ing.Name, "bot-detection")

	config := make(map[string]interface{})

	// Parse allow list (regex patterns)
	if hasAllow && strings.TrimSpace(allowList) != "" {
		patterns := strings.Split(allowList, ",")
		var allowPatterns []string
		for _, p := range patterns {
			p = strings.TrimSpace(p)
			if p != "" {
				allowPatterns = append(allowPatterns, p)
			}
		}
		if len(allowPatterns) > 0 {
			config["allow"] = allowPatterns
			log.Printf("INFO: Ingress %s/%s bot detection allow patterns: %v", ing.Namespace, ing.Name, allowPatterns)
		}
		removeAnnotation(ing, allowKey)
	}

	// Parse deny list (regex patterns)
	if hasDeny && strings.TrimSpace(denyList) != "" {
		patterns := strings.Split(denyList, ",")
		var denyPatterns []string
		for _, p := range patterns {
			p = strings.TrimSpace(p)
			if p != "" {
				denyPatterns = append(denyPatterns, p)
			}
		}
		if len(denyPatterns) > 0 {
			config["deny"] = denyPatterns
			log.Printf("INFO: Ingress %s/%s bot detection deny patterns: %v", ing.Namespace, ing.Name, denyPatterns)
		}
		removeAnnotation(ing, denyKey)
	}

	// Only create plugin if we have config
	if len(config) == 0 {
		return
	}

	plugin := generator.KongPlugin{
		APIVersion: "configuration.konghq.com/v1",
		Kind:       "KongPlugin",
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "bot-detection",
		Config: config,
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)

	log.Printf("INFO: Ingress %s/%s configured bot detection via User-Agent filtering", ing.Namespace, ing.Name)
}
