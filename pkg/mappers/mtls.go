package mappers

import (
	"log"
	"strings"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// MapMTLS handles mTLS client certificate authentication annotations
// NGINX: nginx.ingress.kubernetes.io/auth-tls-verify-client
// NGINX: nginx.ingress.kubernetes.io/auth-tls-match-cn
// NGINX: nginx.ingress.kubernetes.io/auth-tls-verify-depth
// Kong: mtls-auth plugin (Enterprise)
func MapMTLS(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	verifyClientKey := "nginx.ingress.kubernetes.io/auth-tls-verify-client"
	matchCNKey := "nginx.ingress.kubernetes.io/auth-tls-match-cn"
	verifyDepthKey := "nginx.ingress.kubernetes.io/auth-tls-verify-depth"
	secretKey := "nginx.ingress.kubernetes.io/auth-tls-secret"

	verifyClient, hasVerify := ing.Annotations[verifyClientKey]
	matchCN, hasMatchCN := ing.Annotations[matchCNKey]
	verifyDepth, hasDepth := ing.Annotations[verifyDepthKey]
	secret, hasSecret := ing.Annotations[secretKey]

	// Only generate if verify-client is enabled
	if !hasVerify {
		// Clean up related annotations if present
		removeAnnotation(ing, matchCNKey)
		removeAnnotation(ing, verifyDepthKey)
		removeAnnotation(ing, secretKey)
		return
	}

	// Check if mTLS is enabled
	verifyClient = strings.ToLower(strings.TrimSpace(verifyClient))
	if verifyClient != "on" && verifyClient != "optional" && verifyClient != "optional_no_ca" {
		log.Printf("INFO: Ingress %s/%s has auth-tls-verify-client='%s' (disabled). Skipping mTLS configuration.", ing.Namespace, ing.Name, verifyClient)
		removeAnnotation(ing, verifyClientKey)
		removeAnnotation(ing, matchCNKey)
		removeAnnotation(ing, verifyDepthKey)
		removeAnnotation(ing, secretKey)
		return
	}

	// Generate mtls-auth plugin (Enterprise)
	pluginName := generateName(ing.Name, "mtls-auth")

	config := make(map[string]interface{})

	// Configure CA certificates reference
	// In Kong, CA certificates must be uploaded separately
	config["ca_certificates"] = []string{"<PLACEHOLDER_CA_CERT_ID>"}

	// Configure verified common names if specified
	if hasMatchCN && matchCN != "" {
		// matchCN can be a pattern like "*.example.com" or exact match
		commonNames := strings.Split(matchCN, ",")
		var trimmedNames []string
		for _, cn := range commonNames {
			trimmedNames = append(trimmedNames, strings.TrimSpace(cn))
		}
		config["verified_common_names"] = trimmedNames
		log.Printf("INFO: Ingress %s/%s requires client certificate CN to match: %v", ing.Namespace, ing.Name, trimmedNames)
	}

	// Configure verify depth if specified
	if hasDepth && verifyDepth != "" {
		config["verify_depth"] = verifyDepth
	}

	// Handle optional vs required verification
	if verifyClient == "optional" || verifyClient == "optional_no_ca" {
		config["skip_consumer_lookup"] = true
		config["anonymous"] = "" // Allow anonymous access if cert validation fails
		log.Printf("INFO: Ingress %s/%s uses optional client certificate verification. Requests without valid certs will be allowed.", ing.Namespace, ing.Name)
	}

	// Add note about CA secret if specified
	var annotations map[string]string
	if hasSecret {
		annotations = map[string]string{
			"note":             "Requires Kong Enterprise License and CA certificate configuration",
			"nginx-tls-secret": secret,
			"action-required":  "Upload the CA certificate from the secret to Kong and update ca_certificates field with the Kong CA certificate ID",
		}
	} else {
		annotations = map[string]string{
			"note":            "Requires Kong Enterprise License and CA certificate configuration",
			"action-required": "Upload CA certificates to Kong and update ca_certificates field with the Kong CA certificate IDs",
		}
	}

	plugin := generator.KongPlugin{
		APIVersion: "configuration.konghq.com/v1",
		Kind:       "KongPlugin",
		Metadata: generator.ObjectMeta{
			Name:        pluginName,
			Namespace:   ing.Namespace,
			Annotations: annotations,
		},
		Plugin: "mtls-auth",
		Config: config,
	}

	*plugins = append(*plugins, plugin)
	addPluginReference(ing, pluginName)

	log.Printf("ACTION REQUIRED: Ingress %s/%s uses mTLS client authentication. Generated 'mtls-auth' plugin '%s'. This is a Kong Enterprise feature. Steps required:", ing.Namespace, ing.Name, pluginName)
	log.Printf("  1. Upload CA certificate(s) to Kong Gateway")
	if hasSecret {
		log.Printf("  2. Extract CA from Kubernetes secret '%s' and upload to Kong", secret)
	}
	log.Printf("  3. Update the plugin's 'ca_certificates' field with Kong's CA certificate ID(s)")
	log.Printf("  4. Ensure Kong Enterprise license is installed")

	// Handle auth-tls-error-page (informational only)
	errorPageKey := "nginx.ingress.kubernetes.io/auth-tls-error-page"
	if errorPage, hasErrorPage := ing.Annotations[errorPageKey]; hasErrorPage {
		log.Printf("INFO: Ingress %s/%s specifies auth-tls-error-page='%s'. Kong handles mTLS errors with standard HTTP error codes (401, 403). Custom error pages can be configured using Kong's error handling or a custom plugin.", ing.Namespace, ing.Name, errorPage)
		removeAnnotation(ing, errorPageKey)
	}

	removeAnnotation(ing, verifyClientKey)
	removeAnnotation(ing, matchCNKey)
	removeAnnotation(ing, verifyDepthKey)
	removeAnnotation(ing, secretKey)
}
