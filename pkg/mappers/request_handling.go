package mappers

import (
	"log"
	"strconv"
	"strings"
	"unicode"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// mapRequestHandling handles request manipulation annotations
// NGINX: proxy-body-size -> request-size-limiting
// NGINX: custom-header (not standard, but often asked) -> request-transformer
func mapRequestHandling(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	mapBodySize(ing, plugins)
	// Add other header mappers here if needed
}

func mapBodySize(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	key := "nginx.ingress.kubernetes.io/proxy-body-size"
	val, ok := ing.Annotations[key]
	if !ok {
		return
	}

	// Parse NGINX size e.g. "1m", "10M", "100k", "500"
	// Kong supports bytes, kilobytes, megabytes. Default megabytes.

	sizeStr := strings.TrimSpace(strings.ToLower(val))
	var unit string = "megabytes" // Default assumption
	var number int

	// Extract number and unit
	lastChar := sizeStr[len(sizeStr)-1]
	if unicode.IsLetter(rune(lastChar)) {
		unitChar := lastChar
		numPart := sizeStr[:len(sizeStr)-1]

		var err error
		number, err = strconv.Atoi(numPart)
		if err != nil {
			log.Printf("Warning: Could not parse proxy-body-size '%s'", val)
			return
		}

		switch unitChar {
		case 'm':
			unit = "megabytes"
		case 'k':
			unit = "kilobytes"
		case 'b':
			unit = "bytes"
		default:
			// "1g" -> convert to megabytes
			if unitChar == 'g' {
				number = number * 1024
				unit = "megabytes"
			}
		}
	} else {
		// No unit, usually bytes in NGINX? or default?
		// NGINX default is bytes if no unit.
		n, err := strconv.Atoi(sizeStr)
		if err == nil {
			number = n
			unit = "bytes"
		}
	}

	config := map[string]interface{}{
		"allowed_payload_size": number,
		"size_unit":            unit,
	}

	pluginName := generateName(ing.Name, "request-size-limiting")
	plugin := generator.KongPlugin{
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "request-size-limiting",
		Config: config,
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)
	removeAnnotation(ing, key)
}

// MapClientMaxBodySize handles nginx.ingress.kubernetes.io/client-max-body-size
// This is an alias for proxy-body-size - both set request size limits
// Kong: request-size-limiting plugin
func MapClientMaxBodySize(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	key := "nginx.ingress.kubernetes.io/client-max-body-size"

	if val, ok := ing.Annotations[key]; ok {
		// Use the existing proxy-body-size logic
		// Temporarily set proxy-body-size annotation
		proxyKey := "nginx.ingress.kubernetes.io/proxy-body-size"
		ing.Annotations[proxyKey] = val

		// Call existing mapper (mapBodySize)
		mapBodySize(ing, plugins)

		// Clean up client-max-body-size annotation
		removeAnnotation(ing, key)
	}
}
