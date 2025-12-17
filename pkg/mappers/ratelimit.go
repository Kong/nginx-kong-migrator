package mappers

import (
	"log"
	"strconv"
	"strings"

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
	limitByKey := "nginx.ingress.kubernetes.io/limit-by"
	limitByHeaderKey := "nginx.ingress.kubernetes.io/limit-by-header"
	errorCodeKey := "nginx.ingress.kubernetes.io/limit-error-code"
	errorMessageKey := "nginx.ingress.kubernetes.io/limit-error-message"

	rps, hasRPS := ing.Annotations[rpsKey]
	rpm, hasRPM := ing.Annotations[rpmKey]
	limitBy, hasLimitBy := ing.Annotations[limitByKey]
	limitByHeader, hasLimitByHeader := ing.Annotations[limitByHeaderKey]
	errorCode, hasErrorCode := ing.Annotations[errorCodeKey]
	errorMessage, hasErrorMessage := ing.Annotations[errorMessageKey]

	if !hasRPS && !hasRPM {
		// Clean up related annotations
		removeAnnotation(ing, limitByKey)
		removeAnnotation(ing, limitByHeaderKey)
		removeAnnotation(ing, errorCodeKey)
		removeAnnotation(ing, errorMessageKey)
		return
	}

	// Parse rates
	var rpsVal, rpmVal int
	if hasRPS {
		if val, err := strconv.Atoi(rps); err == nil && val > 0 {
			rpsVal = val
		}
	}
	if hasRPM {
		if val, err := strconv.Atoi(rpm); err == nil && val > 0 {
			rpmVal = val
		}
	}

	if rpsVal == 0 && rpmVal == 0 {
		log.Printf("WARNING: Ingress %s/%s has invalid rate limit values", ing.Namespace, ing.Name)
		return
	}

	pluginName := generateName(ing.Name, "rate-limiting")

	config := map[string]interface{}{
		"policy": "local",
	}

	// Add rate limits
	if rpsVal > 0 {
		config["second"] = rpsVal
	}
	if rpmVal > 0 {
		config["minute"] = rpmVal
	}

	// Configure limit_by (what key to use for rate limiting)
	limitByValue := "ip" // Default to IP-based limiting
	if hasLimitBy {
		lb := strings.ToLower(strings.TrimSpace(limitBy))
		// Kong supports: consumer, credential, ip, service, header, path
		validLimitBy := map[string]bool{
			"ip": true, "header": true, "path": true,
			"consumer": true, "credential": true, "service": true,
		}
		if validLimitBy[lb] {
			limitByValue = lb
		} else {
			log.Printf("WARNING: Ingress %s/%s has unsupported limit-by value '%s', using 'ip'", ing.Namespace, ing.Name, lb)
		}
		removeAnnotation(ing, limitByKey)
	}
	config["limit_by"] = limitByValue

	// If limiting by header, specify header name
	if limitByValue == "header" && hasLimitByHeader {
		config["header_name"] = limitByHeader
		removeAnnotation(ing, limitByHeaderKey)
	} else if hasLimitByHeader {
		log.Printf("WARNING: Ingress %s/%s has limit-by-header but limit-by is not 'header'", ing.Namespace, ing.Name)
		removeAnnotation(ing, limitByHeaderKey)
	}

	// Custom error code
	if hasErrorCode {
		if code, err := strconv.Atoi(errorCode); err == nil && code >= 100 && code <= 599 {
			config["error_code"] = code
		}
		removeAnnotation(ing, errorCodeKey)
	}

	// Custom error message
	if hasErrorMessage && strings.TrimSpace(errorMessage) != "" {
		config["error_message"] = errorMessage
		removeAnnotation(ing, errorMessageKey)
	}

	plugin := generator.KongPlugin{
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "rate-limiting",
		Config: config,
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)

	log.Printf("INFO: Ingress %s/%s configured rate limiting (limit_by: %s)", ing.Namespace, ing.Name, limitByValue)
	removeAnnotation(ing, rpsKey)
	removeAnnotation(ing, rpmKey)
}
