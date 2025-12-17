package mappers

import (
	"log"
	"strconv"
	"strings"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// MapMaintenanceMode handles maintenance mode annotation
// NGINX: Custom annotation (not standard) - returns custom response
// Kong: request-termination plugin
func MapMaintenanceMode(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	key := "nginx.ingress.kubernetes.io/maintenance-mode"
	statusKey := "nginx.ingress.kubernetes.io/maintenance-status-code"
	messageKey := "nginx.ingress.kubernetes.io/maintenance-message"

	enabled, hasMode := ing.Annotations[key]

	if !hasMode || strings.ToLower(strings.TrimSpace(enabled)) != "true" {
		// Clean up related annotations
		removeAnnotation(ing, statusKey)
		removeAnnotation(ing, messageKey)
		return
	}

	pluginName := generateName(ing.Name, "maintenance-mode")

	// Default maintenance response
	statusCode := 503
	message := "Service temporarily unavailable for maintenance"

	// Parse custom status code if provided
	if status, ok := ing.Annotations[statusKey]; ok {
		if code, err := strconv.Atoi(status); err == nil && code >= 100 && code <= 599 {
			statusCode = code
		}
		removeAnnotation(ing, statusKey)
	}

	// Parse custom message if provided
	if msg, ok := ing.Annotations[messageKey]; ok {
		if strings.TrimSpace(msg) != "" {
			message = msg
		}
		removeAnnotation(ing, messageKey)
	}

	plugin := generator.KongPlugin{
		APIVersion: "configuration.konghq.com/v1",
		Kind:       "KongPlugin",
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "request-termination",
		Config: map[string]interface{}{
			"status_code": statusCode,
			"message":     message,
		},
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)

	log.Printf("INFO: Ingress %s/%s enabled maintenance mode. Requests will return %d without proxying upstream.", ing.Namespace, ing.Name, statusCode)
	removeAnnotation(ing, key)
}

// MapDefaultBackendResponse handles custom default backend responses
// NGINX: default-backend annotation can specify a service for fallback
// Kong: request-termination plugin for custom responses
func MapDefaultBackendResponse(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	statusKey := "nginx.ingress.kubernetes.io/default-backend-status-code"
	messageKey := "nginx.ingress.kubernetes.io/default-backend-message"
	contentTypeKey := "nginx.ingress.kubernetes.io/default-backend-content-type"

	// Only create if custom response annotations are present
	status, hasStatus := ing.Annotations[statusKey]
	message, hasMessage := ing.Annotations[messageKey]
	contentType, hasContentType := ing.Annotations[contentTypeKey]

	if !hasStatus && !hasMessage {
		// No custom default response configured
		removeAnnotation(ing, contentTypeKey)
		return
	}

	pluginName := generateName(ing.Name, "default-response")

	// Default 404 response
	statusCode := 404
	defaultMessage := "Not Found"

	// Parse custom status code
	if hasStatus {
		if code, err := strconv.Atoi(status); err == nil && code >= 100 && code <= 599 {
			statusCode = code
		}
		removeAnnotation(ing, statusKey)
	}

	// Use custom message if provided
	responseMessage := defaultMessage
	if hasMessage && strings.TrimSpace(message) != "" {
		responseMessage = message
		removeAnnotation(ing, messageKey)
	}

	config := map[string]interface{}{
		"status_code": statusCode,
		"message":     responseMessage,
	}

	// Add content type if specified
	if hasContentType && strings.TrimSpace(contentType) != "" {
		config["content_type"] = contentType
		removeAnnotation(ing, contentTypeKey)
	}

	plugin := generator.KongPlugin{
		APIVersion: "configuration.konghq.com/v1",
		Kind:       "KongPlugin",
		Metadata: generator.ObjectMeta{
			Name:      pluginName,
			Namespace: ing.Namespace,
		},
		Plugin: "request-termination",
		Config: config,
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)

	log.Printf("INFO: Ingress %s/%s configured custom default backend response (%d).", ing.Namespace, ing.Name, statusCode)
}
