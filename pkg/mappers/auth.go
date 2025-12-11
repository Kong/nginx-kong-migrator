package mappers

import (
	"log"

	"nginx-kong-migrator/pkg/generator"

	networkingv1 "k8s.io/api/networking/v1"
)

// mapAuth handles authentication annotations
// NGINX: nginx.ingress.kubernetes.io/auth-type
// NGINX: nginx.ingress.kubernetes.io/auth-secret
// KONG: KongPlugin (basic-auth)
func mapAuth(ing *networkingv1.Ingress, plugins *[]generator.KongPlugin) {
	typeKey := "nginx.ingress.kubernetes.io/auth-type"
	secretKey := "nginx.ingress.kubernetes.io/auth-secret"

	authType, hasType := ing.Annotations[typeKey]
	secretName, hasSecret := ing.Annotations[secretKey]

	// Handle Basic Auth
	if hasType && authType == "basic" {
		// Create Plugin
		pluginName := generateName(ing.Name, "basic-auth")

		// Note: basic-auth plugin in Kong usually doesn't need config in the plugin itself
		// to point to a specific secret. Instead, credentials are created as KongConsumers
		// and associated with the secret.
		// However, for migration visibility, creating the plugin is the first step to enforcing auth.

		plugin := generator.KongPlugin{
			Metadata: generator.ObjectMeta{
				Name:      pluginName,
				Namespace: ing.Namespace,
			},
			Plugin: "basic-auth",
			// basic-auth plugin often has empty config to start
			Config: map[string]interface{}{},
		}

		*plugins = append(*plugins, plugin)
		addPluginReference(ing, pluginName)

		removeAnnotation(ing, typeKey)

		if hasSecret {
			// We can't automatically migrate the secret content to Kong Consumers easily without
			// reading actual K8s Secrets which we are avoiding in this offline tool.
			// We will leave a warning log or comment.
			log.Printf("WARNING: Ingress %s/%s uses 'auth-secret: %s'. You must manually create Kong Consumers for these credentials.",
				ing.Namespace, ing.Name, secretName)

			// We choose NOT to remove the auth-secret annotation so the user can see it as reference?
			// Or we remove it to be clean. Let's remove it but log the warning.
			removeAnnotation(ing, secretKey)
		}
	}

	// Handle OIDC / OAuth2 Proxy (auth-signin) -> openid-connect (Enterprise)
	signinKey := "nginx.ingress.kubernetes.io/auth-signin"
	if signinVal, ok := ing.Annotations[signinKey]; ok {
		pluginName := generateName(ing.Name, "oidc")

		log.Printf("ACTION REQUIRED: Ingress %s/%s uses 'auth-signin'. Generated 'openid-connect' plugin '%s'. This is a Kong Enterprise feature. You must fill in the placeholder Client ID/Secret and Issuer.", ing.Namespace, ing.Name, pluginName)

		plugin := generator.KongPlugin{
			Metadata: generator.ObjectMeta{
				Name:      pluginName,
				Namespace: ing.Namespace,
				Annotations: map[string]string{
					"note": "Requires Kong Enterprise License",
				},
			},
			Plugin: "openid-connect",
			Config: map[string]interface{}{
				"issuer":              "https://<PLACEHOLDER_ISSUER>",
				"client_id":           []string{"<PLACEHOLDER_CLIENT_ID>"},
				"client_secret":       []string{"<PLACEHOLDER_CLIENT_SECRET>"},
				"redirect_uri":        []string{"https://" + ing.Spec.Rules[0].Host + "/"},
				"scopes":              []string{"openid", "profile", "email"},
				"login_redirect_mode": "header",
				// "_comment":          "Migrated from NGINX auth-signin: " + signinVal, // Removed for KIC validation
			},
		}
		_ = signinVal // Keep variable used to satisfy compiler

		*plugins = append(*plugins, plugin)
		addPluginReference(ing, pluginName)
		removeAnnotation(ing, signinKey)
		removeAnnotation(ing, "nginx.ingress.kubernetes.io/auth-url") // Remove partner annotation
	}
}
