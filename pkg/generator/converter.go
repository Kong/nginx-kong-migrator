package generator

import (
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
)

// ConvertToHTTPRoute converts a Kubernetes Ingress to Gateway API HTTPRoute
func ConvertToHTTPRoute(ing *networkingv1.Ingress, gatewayName, gatewayNamespace string) *HTTPRoute {
	route := &HTTPRoute{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "HTTPRoute",
		Metadata: ObjectMeta{
			Name:      ing.Name,
			Namespace: ing.Namespace,
		},
		Spec: HTTPRouteSpec{
			ParentRefs: []ParentReference{
				{
					Name:      gatewayName,
					Namespace: &gatewayNamespace,
				},
			},
		},
	}

	// Extract hostnames from rules
	var hostnames []string
	hostnameSet := make(map[string]bool)
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" && !hostnameSet[rule.Host] {
			hostnames = append(hostnames, rule.Host)
			hostnameSet[rule.Host] = true
		}
	}
	route.Spec.Hostnames = hostnames

	// Extract Gateway API native features from annotations
	rewriteTarget := ing.Annotations["nginx.ingress.kubernetes.io/rewrite-target"]
	corsEnabled := ing.Annotations["nginx.ingress.kubernetes.io/enable-cors"]
	corsOrigins := ing.Annotations["nginx.ingress.kubernetes.io/cors-allow-origin"]
	canaryEnabled := ing.Annotations["nginx.ingress.kubernetes.io/canary"]
	canaryWeight := ing.Annotations["nginx.ingress.kubernetes.io/canary-weight"]

	// Convert rules
	for _, ingressRule := range ing.Spec.Rules {
		if ingressRule.HTTP == nil {
			continue
		}

		for _, path := range ingressRule.HTTP.Paths {
			rule := HTTPRouteRule{
				Matches: []HTTPRouteMatch{
					{
						Path: convertPathMatch(path),
					},
				},
			}

			// Add URL Rewrite filter if rewrite-target is specified
			if rewriteTarget != "" {
				rule.Filters = append(rule.Filters, HTTPRouteFilter{
					Type: "URLRewrite",
					URLRewrite: &HTTPURLRewriteFilter{
						Path: &HTTPPathModifier{
							Type:            "ReplaceFullPath",
							ReplaceFullPath: &rewriteTarget,
						},
					},
				})
			}

			// Add CORS responseHeaderModifier if enabled
			if corsEnabled == "true" {
				corsFilter := HTTPRouteFilter{
					Type: "ResponseHeaderModifier",
					ResponseHeaderModifier: &HeaderModifier{
						Add: []HTTPHeader{
							{Name: "Access-Control-Allow-Origin", Value: corsOrigins},
							{Name: "Access-Control-Allow-Methods", Value: "GET, POST, OPTIONS"},
							{Name: "Access-Control-Allow-Headers", Value: "DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization"},
						},
					},
				}
				if corsOrigins == "" {
					corsFilter.ResponseHeaderModifier.Add[0].Value = "*"
				}
				rule.Filters = append(rule.Filters, corsFilter)
			}

			// Handle canary routing with weighted backends
			if canaryEnabled == "true" && canaryWeight != "" {
				// TODO: Extract canary service from annotations or naming convention
				// For now, assume v2 variant based on service name
				primaryService := path.Backend.Service.Name
				canaryService := primaryService + "-canary" // or extract from annotation

				weight, _ := parseWeight(canaryWeight)
				primaryWeight := int32(100 - weight)

				rule.BackendRefs = []HTTPBackendRef{
					{
						BackendRef: BackendRef{
							Name:   primaryService,
							Port:   ptr(int32(path.Backend.Service.Port.Number)),
							Weight: &primaryWeight,
						},
					},
					{
						BackendRef: BackendRef{
							Name:   canaryService,
							Port:   ptr(int32(path.Backend.Service.Port.Number)),
							Weight: &weight,
						},
					},
				}
			} else {
				// Standard backend reference
				rule.BackendRefs = []HTTPBackendRef{
					{
						BackendRef: BackendRef{
							Name: path.Backend.Service.Name,
							Port: ptr(int32(path.Backend.Service.Port.Number)),
						},
					},
				}
			}

			route.Spec.Rules = append(route.Spec.Rules, rule)
		}
	}

	return route
}

// parseWeight parses canary weight string to int32
func parseWeight(weight string) (int32, error) {
	var w int
	_, err := fmt.Sscanf(weight, "%d", &w)
	if err != nil || w < 0 || w > 100 {
		return 0, fmt.Errorf("invalid weight: %s", weight)
	}
	return int32(w), nil
}

// convertPathMatch converts Ingress PathType to Gateway API HTTPPathMatch
func convertPathMatch(path networkingv1.HTTPIngressPath) *HTTPPathMatch {
	if path.Path == "" {
		// Default to prefix match on "/"
		return &HTTPPathMatch{
			Type:  "PathPrefix",
			Value: ptr("/"),
		}
	}

	match := &HTTPPathMatch{
		Value: &path.Path,
	}

	// Map Ingress PathType to Gateway API path match type
	if path.PathType != nil {
		switch *path.PathType {
		case networkingv1.PathTypeExact:
			match.Type = "Exact"
		case networkingv1.PathTypePrefix:
			match.Type = "PathPrefix"
		default:
			// ImplementationSpecific - default to PathPrefix
			match.Type = "PathPrefix"
		}
	} else {
		// Default to PathPrefix if not specified
		match.Type = "PathPrefix"
	}

	return match
}

// ptr is a helper function to create a pointer to a value
func ptr[T any](v T) *T {
	return &v
}
