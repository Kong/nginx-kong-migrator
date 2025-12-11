package mappers

import (
	"strconv"
	"strings"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// mapTimeouts handles timeout annotations
// NGINX: nginx.ingress.kubernetes.io/proxy-connect-timeout -> konghq.com/connect-timeout
// NGINX: nginx.ingress.kubernetes.io/proxy-read-timeout    -> konghq.com/read-timeout
// NGINX: nginx.ingress.kubernetes.io/proxy-send-timeout    -> konghq.com/write-timeout
func mapTimeouts(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	// Map: Key -> Target Key
	mappings := map[string]string{
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "konghq.com/connect-timeout",
		"nginx.ingress.kubernetes.io/proxy-read-timeout":    "konghq.com/read-timeout",
		"nginx.ingress.kubernetes.io/proxy-send-timeout":    "konghq.com/write-timeout",
	}

	for nginxKey, kongKey := range mappings {
		if val, ok := ing.Annotations[nginxKey]; ok {
			// NGINX values are often in seconds (e.g., "60").
			// Kong annotations expect milliseconds.
			// However, looking at Kong docs, `konghq.com/connect-timeout` takes an integer in milliseconds.
			// But wait, KIC docs say: "The value is in milliseconds."
			// NGINX docs say: "Sets the timeout ... time in seconds".

			// So we need to multiply by 1000.

			// Handle trailing 's' if present (e.g. "60s")
			cleanVal := strings.TrimSuffix(val, "s")

			if i, err := strconv.Atoi(cleanVal); err == nil {
				ms := i * 1000
				addAnnotation(ing, kongKey, strconv.Itoa(ms))
				removeAnnotation(ing, nginxKey)
			}
		}
	}
}
