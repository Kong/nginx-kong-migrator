package mappers

import (
	"log"
	"strconv"
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
	originsKey := "nginx.ingress.kubernetes.io/cors-allow-origin"
	methodsKey := "nginx.ingress.kubernetes.io/cors-allow-methods"
	headersKey := "nginx.ingress.kubernetes.io/cors-allow-headers"
	credentialsKey := "nginx.ingress.kubernetes.io/cors-allow-credentials"
	maxAgeKey := "nginx.ingress.kubernetes.io/cors-max-age"
	exposeHeadersKey := "nginx.ingress.kubernetes.io/cors-expose-headers"
	preflightContinueKey := "nginx.ingress.kubernetes.io/cors-preflight-continue"

	enabled, hasEnable := ing.Annotations[enableKey]

	if !hasEnable || strings.ToLower(enabled) != "true" {
		// Clean up all CORS-related annotations
		removeAnnotation(ing, originsKey)
		removeAnnotation(ing, methodsKey)
		removeAnnotation(ing, headersKey)
		removeAnnotation(ing, credentialsKey)
		removeAnnotation(ing, maxAgeKey)
		removeAnnotation(ing, exposeHeadersKey)
		removeAnnotation(ing, preflightContinueKey)
		return
	}

	pluginName := generateName(ing.Name, "cors")

	config := map[string]interface{}{
		"origins": []string{"*"}, // Default: allow all
	}

	// Parse allowed origins
	if origins, ok := ing.Annotations[originsKey]; ok {
		originList := strings.Split(origins, ",")
		var parsedOrigins []string
		for _, o := range originList {
			o = strings.TrimSpace(o)
			if o != "" {
				parsedOrigins = append(parsedOrigins, o)
			}
		}
		if len(parsedOrigins) > 0 {
			config["origins"] = parsedOrigins
		}
		removeAnnotation(ing, originsKey)
	}

	// Parse allowed methods
	if methods, ok := ing.Annotations[methodsKey]; ok {
		methodList := strings.Split(methods, ",")
		var parsedMethods []string
		for _, m := range methodList {
			m = strings.TrimSpace(strings.ToUpper(m))
			if m != "" {
				parsedMethods = append(parsedMethods, m)
			}
		}
		if len(parsedMethods) > 0 {
			config["methods"] = parsedMethods
		}
		removeAnnotation(ing, methodsKey)
	}

	// Parse allowed headers
	if headers, ok := ing.Annotations[headersKey]; ok {
		headerList := strings.Split(headers, ",")
		var parsedHeaders []string
		for _, h := range headerList {
			h = strings.TrimSpace(h)
			if h != "" {
				parsedHeaders = append(parsedHeaders, h)
			}
		}
		if len(parsedHeaders) > 0 {
			config["headers"] = parsedHeaders
		}
		removeAnnotation(ing, headersKey)
	}

	// Parse exposed headers (NEW - enhancement)
	if exposeHeaders, ok := ing.Annotations[exposeHeadersKey]; ok {
		exposeList := strings.Split(exposeHeaders, ",")
		var parsedExposeHeaders []string
		for _, h := range exposeList {
			h = strings.TrimSpace(h)
			if h != "" {
				parsedExposeHeaders = append(parsedExposeHeaders, h)
			}
		}
		if len(parsedExposeHeaders) > 0 {
			config["exposed_headers"] = parsedExposeHeaders
		}
		removeAnnotation(ing, exposeHeadersKey)
	}

	// Parse credentials
	if creds, ok := ing.Annotations[credentialsKey]; ok {
		if strings.ToLower(creds) == "true" {
			config["credentials"] = true
		}
		removeAnnotation(ing, credentialsKey)
	}

	// Parse max age
	if maxAge, ok := ing.Annotations[maxAgeKey]; ok {
		if age, err := strconv.Atoi(maxAge); err == nil && age > 0 {
			config["max_age"] = age
		}
		removeAnnotation(ing, maxAgeKey)
	}

	// Parse preflight continue (NEW - enhancement)
	if preflightContinue, ok := ing.Annotations[preflightContinueKey]; ok {
		if strings.ToLower(preflightContinue) == "true" {
			config["preflight_continue"] = true
			log.Printf("INFO: Ingress %s/%s will proxy OPTIONS preflight requests to upstream", ing.Namespace, ing.Name)
		}
		removeAnnotation(ing, preflightContinueKey)
	}

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
