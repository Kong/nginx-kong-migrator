package analyzer

import (
	"sort"
	"strings"

	"nginx-kong-migrator/pkg/generator"
	"nginx-kong-migrator/pkg/mappers"

	networkingv1 "k8s.io/api/networking/v1"
)

// Status represents the migration readiness of an Ingress.
type Status int

const (
	StatusGreen  Status = iota // Already migrated — not a nginx ingress
	StatusYellow               // Ready to migrate — nginx ingress (clean or with action notes requiring manual followup)
	StatusRed                  // Manual — nginx ingress with annotations that have no Kong equivalent
)

// IngressAnalysis holds the migration analysis result for an Ingress.
type IngressAnalysis struct {
	Name             string   `json:"name"`
	Namespace        string   `json:"namespace"`
	Hosts            []string `json:"hosts"`
	Status           Status   `json:"status"`
	ActionNotes      []string `json:"actionNotes"`
	UnmigratedKeys   []string `json:"unmigratedKeys"`
	GeneratedPlugins []string `json:"generatedPlugins"`
	AnnotationCount  int      `json:"annotationCount"`
}

var actionRequiredAnnotations = map[string]string{
	"nginx.ingress.kubernetes.io/affinity":               "Session affinity (cookie) requires manual KongConsumer/KongPlugin configuration",
	"nginx.ingress.kubernetes.io/auth-secret":            "Basic auth requires manual KongConsumer and credentials setup",
	"nginx.ingress.kubernetes.io/auth-signin":            "OIDC external auth requires KongPlugin oauth2 or oidc configuration",
	"nginx.ingress.kubernetes.io/auth-tls-verify-client": "mTLS client verification requires manual certificate setup",
	"nginx.ingress.kubernetes.io/limit-connections":      "Connection limiting requires manual KongPlugin rate-limiting configuration",
	"nginx.ingress.kubernetes.io/mirror-target":          "Request mirroring requires manual KongPlugin configuration",
}

// getIngressClass returns the effective ingress class from spec or annotation.
func getIngressClass(ing networkingv1.Ingress) string {
	if ing.Spec.IngressClassName != nil {
		return *ing.Spec.IngressClassName
	}
	return ing.Annotations["kubernetes.io/ingress.class"]
}

// isNginxIngress returns true if the ingress is managed by nginx.
func isNginxIngress(ing networkingv1.Ingress) bool {
	if getIngressClass(ing) == "nginx" {
		return true
	}
	for key := range ing.Annotations {
		if strings.HasPrefix(key, "nginx.ingress.kubernetes.io/") {
			return true
		}
	}
	return false
}

// Analyze performs migration analysis on an Ingress without modifying the original.
func Analyze(ing networkingv1.Ingress) IngressAnalysis {
	// Count original nginx annotations before any transformation
	annotationCount := 0
	for key := range ing.Annotations {
		if strings.HasPrefix(key, "nginx.ingress.kubernetes.io/") {
			annotationCount++
		}
	}

	// Extract unique hosts from ingress rules
	var hosts []string
	hostSet := make(map[string]bool)
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" && !hostSet[rule.Host] {
			hosts = append(hosts, rule.Host)
			hostSet[rule.Host] = true
		}
	}

	// Not a nginx ingress → already migrated (green)
	if !isNginxIngress(ing) {
		return IngressAnalysis{
			Name:            ing.Name,
			Namespace:       ing.Namespace,
			Hosts:           hosts,
			Status:          StatusGreen,
			AnnotationCount: annotationCount,
		}
	}

	// Deep-copy the ingress annotations so Apply() does not mutate the original
	ingCopy := ing
	ingCopy.Annotations = make(map[string]string)
	for k, v := range ing.Annotations {
		ingCopy.Annotations[k] = v
	}

	// Record which action-required annotations are present before Apply
	var actionNotes []string
	for annotation, note := range actionRequiredAnnotations {
		if _, ok := ingCopy.Annotations[annotation]; ok {
			actionNotes = append(actionNotes, note)
		}
	}

	// Run Apply to transform annotations
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	mappers.Apply(&ingCopy, &plugins, &kongIngresses, &upstreamPolicies)

	// Collect any remaining nginx annotations after Apply (unmigrated)
	var unmigratedKeys []string
	for key := range ingCopy.Annotations {
		if strings.HasPrefix(key, "nginx.ingress.kubernetes.io/") {
			unmigratedKeys = append(unmigratedKeys, key)
		}
	}

	// Collect generated plugin names
	var generatedPlugins []string
	for _, p := range plugins {
		generatedPlugins = append(generatedPlugins, p.Metadata.Name)
	}

	// Determine status: manual only if truly unmigrated (no Kong equivalent);
	// action-note ingresses are still migratable (Yellow) but require manual followup.
	var status Status
	switch {
	case len(unmigratedKeys) > 0:
		status = StatusRed // Manual — no Kong equivalent
	default:
		status = StatusYellow // Ready — clean or with action notes
	}

	sort.Strings(actionNotes)
	sort.Strings(unmigratedKeys)

	return IngressAnalysis{
		Name:             ing.Name,
		Namespace:        ing.Namespace,
		Hosts:            hosts,
		Status:           status,
		ActionNotes:      actionNotes,
		UnmigratedKeys:   unmigratedKeys,
		GeneratedPlugins: generatedPlugins,
		AnnotationCount:  annotationCount,
	}
}
