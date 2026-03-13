package mappers

import (
	"log"

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
