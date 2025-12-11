package mappers

import (
	"log"
	"regexp"
	"strings"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// mapExternalAuth handles external authentication annotations
func mapExternalAuth(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	urlKey := "nginx.ingress.kubernetes.io/auth-url"
	signinKey := "nginx.ingress.kubernetes.io/auth-signin"

	_, hasURL := ing.Annotations[urlKey]
	_, hasSignin := ing.Annotations[signinKey]

	if hasURL || hasSignin {
		log.Printf("WARNING: Ingress %s/%s uses 'auth-url' (External Auth). Kong typically uses 'openid-connect' (Enterprise) or specific community plugins for this. Manual migration required.", ing.Namespace, ing.Name)
		removeAnnotation(ing, urlKey)
		removeAnnotation(ing, signinKey)
	}
}

// mapSnippetHeaders parses configuration-snippet for headers
func mapSnippetHeaders(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	key := "nginx.ingress.kubernetes.io/configuration-snippet"
	val, ok := ing.Annotations[key]
	if !ok {
		return
	}

	// 1. Request Headers: proxy_set_header Key Val;
	// Regex: proxy_set_header\s+([a-zA-Z0-9-]+)\s+"?([^";]+)"?;
	reProxy := regexp.MustCompile(`proxy_set_header\s+([a-zA-Z0-9-]+)\s+"?([^";]+)"?;`)
	matchesProxy := reProxy.FindAllStringSubmatch(val, -1)

	if len(matchesProxy) > 0 {
		headers := []string{}
		for _, match := range matchesProxy {
			if len(match) == 3 {
				headerName := match[1]
				headerVal := match[2]
				if strings.HasPrefix(headerVal, "$") {
					continue
				}
				headers = append(headers, headerName+":"+headerVal)
			}
		}
		if len(headers) > 0 {
			pluginName := generateName(ing.Name, "request-transformer-headers")
			plugin := generator.KongPlugin{
				Metadata: generator.ObjectMeta{Name: pluginName, Namespace: ing.Namespace},
				Plugin:   "request-transformer",
				Config: map[string]interface{}{
					"add": map[string]interface{}{"headers": headers},
				},
			}
			*plugins = append(*plugins, plugin)
			addPluginReference(ing, pluginName)
		}
	}

	// 2. Response Headers: more_set_headers "Key: Val";
	// Regex needs to handle potential whitespace/newlines and quotes
	reMore := regexp.MustCompile(`more_set_headers\s+"?([^":]+):\s*([^";]+)"?;`)
	matchesMore := reMore.FindAllStringSubmatch(val, -1)

	if len(matchesMore) > 0 {
		headers := []string{}
		for _, match := range matchesMore {
			if len(match) == 3 {
				headerName := match[1]
				headerVal := match[2]
				headers = append(headers, headerName+":"+headerVal)
			}
		}
		if len(headers) > 0 {
			pluginName := generateName(ing.Name, "response-transformer-headers")
			plugin := generator.KongPlugin{
				Metadata: generator.ObjectMeta{Name: pluginName, Namespace: ing.Namespace},
				Plugin:   "response-transformer",
				Config: map[string]interface{}{
					"add": map[string]interface{}{"headers": headers},
				},
			}
			*plugins = append(*plugins, plugin)
			addPluginReference(ing, pluginName)
		}
	}
}
