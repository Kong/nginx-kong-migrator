package ui

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"nginx-kong-migrator/pkg/analyzer"
	"nginx-kong-migrator/pkg/generator"
	"nginx-kong-migrator/pkg/mappers"

	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

//go:embed templates/*
var templateFS embed.FS

type srv struct {
	namespace string
	client    kubernetes.Interface
	dynClient dynamic.Interface
}

var (
	kongPluginGVR = schema.GroupVersionResource{
		Group:    "configuration.konghq.com",
		Version:  "v1",
		Resource: "kongplugins",
	}
	kongUpstreamPolicyGVR = schema.GroupVersionResource{
		Group:    "configuration.konghq.com",
		Version:  "v1beta1",
		Resource: "kongupstreampolicies",
	}
)

// Start builds Kubernetes clients and starts the HTTP server on the given port.
func Start(port int, namespace string, kubeconfig string) error {
	config, err := buildConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	s := &srv{namespace: namespace, client: client, dynClient: dynClient}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/ingresses", s.handleIngresses)
	mux.HandleFunc("/api/ingress/", s.handleIngressDownload)
	mux.HandleFunc("/api/migrate", s.handleMigrate)
	mux.HandleFunc("/api/copy-to-namespace", s.handleCopyToNamespace)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Dashboard ready at http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		return clientcmd.BuildConfigFromFlags("", kc)
	}
	if home, err := os.UserHomeDir(); err == nil {
		defaultKC := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(defaultKC); err == nil {
			return clientcmd.BuildConfigFromFlags("", defaultKC)
		}
	}
	return rest.InClusterConfig()
}

func (s *srv) fetchIngresses(ctx context.Context) ([]networkingv1.Ingress, error) {
	list, err := s.client.NetworkingV1().Ingresses(s.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *srv) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := templateFS.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *srv) handleIngresses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ingresses, err := s.fetchIngresses(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	analyses := make([]analyzer.IngressAnalysis, 0, len(ingresses))
	for _, ing := range ingresses {
		analyses = append(analyses, analyzer.Analyze(ing))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analyses)
}

// handleIngressDownload serves: GET /api/ingress/{ns}/{name}/download?format=...
func (s *srv) handleIngressDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/ingress/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 || parts[2] != "download" {
		http.NotFound(w, r)
		return
	}
	ns, name := parts[0], parts[1]
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "kong-ingress"
	}

	ing, err := s.client.NetworkingV1().Ingresses(ns).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("ingress not found: %v", err), http.StatusNotFound)
		return
	}

	ingCopy := deepCopyIngress(ing)
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	mappers.Apply(ingCopy, &plugins, &kongIngresses, &upstreamPolicies)

	var buf bytes.Buffer
	if format == "gateway-api" {
		writeGatewayAPIToWriter(&buf, []networkingv1.Ingress{*ingCopy}, plugins)
	} else {
		writeOutputToWriter(&buf, []networkingv1.Ingress{*ingCopy}, plugins, kongIngresses, upstreamPolicies)
	}

	filename := fmt.Sprintf("%s-%s-%s.yaml", ns, name, format)
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write(buf.Bytes())
}

type migrateRequest struct {
	Ingresses    []ingressRef `json:"ingresses"`
	IngressClass string       `json:"ingressClass"`
}

type ingressRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// MigrateResult is the per-ingress outcome returned by /api/migrate.
type MigrateResult struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

func (s *srv) handleMigrate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req migrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if req.IngressClass == "" {
		req.IngressClass = "kong"
	}

	results := make([]MigrateResult, 0, len(req.Ingresses))
	for _, ref := range req.Ingresses {
		results = append(results, s.migrateOne(r.Context(), ref.Namespace, ref.Name, req.IngressClass))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *srv) migrateOne(ctx context.Context, ns, name, ingressClass string) MigrateResult {
	result := MigrateResult{Namespace: ns, Name: name}

	ing, err := s.client.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		result.Error = fmt.Sprintf("failed to get ingress: %v", err)
		return result
	}

	// Only migrate Ready (yellow) ingresses
	analysis := analyzer.Analyze(*ing)
	if analysis.Status != analyzer.StatusYellow {
		result.Error = "cannot migrate: only Ready ingresses can be migrated"
		return result
	}

	ingCopy := deepCopyIngress(ing)
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	mappers.Apply(ingCopy, &plugins, &kongIngresses, &upstreamPolicies)

	// Create or update KongPlugin CRDs
	for _, plugin := range plugins {
		if plugin.APIVersion == "" {
			plugin.APIVersion = "configuration.konghq.com/v1"
		}
		if plugin.Kind == "" {
			plugin.Kind = "KongPlugin"
		}
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": plugin.APIVersion,
				"kind":       plugin.Kind,
				"metadata":   map[string]interface{}{"name": plugin.Metadata.Name, "namespace": ns},
				"plugin":     plugin.Plugin,
				"config":     plugin.Config,
			},
		}
		if err := s.applyUnstructured(ctx, kongPluginGVR, ns, obj); err != nil {
			result.Error = fmt.Sprintf("failed to apply KongPlugin %s: %v", plugin.Metadata.Name, err)
			return result
		}
	}

	// Create or update KongUpstreamPolicy CRDs
	for _, policy := range upstreamPolicies {
		if policy.APIVersion == "" {
			policy.APIVersion = "configuration.konghq.com/v1beta1"
		}
		if policy.Kind == "" {
			policy.Kind = "KongUpstreamPolicy"
		}
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": policy.APIVersion,
				"kind":       policy.Kind,
				"metadata":   map[string]interface{}{"name": policy.Metadata.Name, "namespace": ns},
				"spec":       policy.Spec,
			},
		}
		if err := s.applyUnstructured(ctx, kongUpstreamPolicyGVR, ns, obj); err != nil {
			result.Error = fmt.Sprintf("failed to apply KongUpstreamPolicy %s: %v", policy.Metadata.Name, err)
			return result
		}
	}

	// Patch the Ingress: set ingressClassName and apply migrated annotations
	ingCopy.Spec.IngressClassName = &ingressClass
	if _, err = s.client.NetworkingV1().Ingresses(ns).Update(ctx, ingCopy, metav1.UpdateOptions{}); err != nil {
		result.Error = fmt.Sprintf("failed to update ingress: %v", err)
		return result
	}

	result.Success = true
	return result
}

type copyToNsRequest struct {
	Ingresses       []ingressRef `json:"ingresses"`
	IngressClass    string       `json:"ingressClass"`
	TargetNamespace string       `json:"targetNamespace"`
}

// CopyToNsResult is the per-ingress outcome returned by /api/copy-to-namespace.
type CopyToNsResult struct {
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	TargetNamespace string `json:"targetNamespace"`
	Success         bool   `json:"success"`
	Error           string `json:"error,omitempty"`
}

func (s *srv) handleCopyToNamespace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req copyToNsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if req.IngressClass == "" {
		req.IngressClass = "kong"
	}
	if req.TargetNamespace == "" {
		http.Error(w, "targetNamespace is required", http.StatusBadRequest)
		return
	}

	results := make([]CopyToNsResult, 0, len(req.Ingresses))
	for _, ref := range req.Ingresses {
		results = append(results, s.copyToNsOne(r.Context(), ref.Namespace, ref.Name, req.IngressClass, req.TargetNamespace))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *srv) copyToNsOne(ctx context.Context, ns, name, ingressClass, targetNs string) CopyToNsResult {
	result := CopyToNsResult{Namespace: ns, Name: name, TargetNamespace: targetNs}

	ing, err := s.client.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		result.Error = fmt.Sprintf("failed to get ingress: %v", err)
		return result
	}

	// Only Ready (yellow) ingresses can be copied
	analysis := analyzer.Analyze(*ing)
	if analysis.Status != analyzer.StatusYellow {
		result.Error = "cannot copy: only Ready ingresses can be copied to a namespace"
		return result
	}

	ingCopy := deepCopyIngress(ing)
	var plugins []generator.KongPlugin
	var kongIngresses []generator.KongIngress
	var upstreamPolicies []generator.KongUpstreamPolicy
	mappers.Apply(ingCopy, &plugins, &kongIngresses, &upstreamPolicies)

	// Set target metadata — clear server-side fields
	ingCopy.Namespace = targetNs
	ingCopy.ResourceVersion = ""
	ingCopy.UID = ""
	ingCopy.CreationTimestamp = metav1.Time{}
	ingCopy.Spec.IngressClassName = &ingressClass

	// Create or update KongPlugin CRDs in target namespace
	for _, plugin := range plugins {
		if plugin.APIVersion == "" {
			plugin.APIVersion = "configuration.konghq.com/v1"
		}
		if plugin.Kind == "" {
			plugin.Kind = "KongPlugin"
		}
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": plugin.APIVersion,
				"kind":       plugin.Kind,
				"metadata":   map[string]interface{}{"name": plugin.Metadata.Name, "namespace": targetNs},
				"plugin":     plugin.Plugin,
				"config":     plugin.Config,
			},
		}
		if err := s.applyUnstructured(ctx, kongPluginGVR, targetNs, obj); err != nil {
			result.Error = fmt.Sprintf("failed to apply KongPlugin %s: %v", plugin.Metadata.Name, err)
			return result
		}
	}

	// Create or update KongUpstreamPolicy CRDs in target namespace
	for _, policy := range upstreamPolicies {
		if policy.APIVersion == "" {
			policy.APIVersion = "configuration.konghq.com/v1beta1"
		}
		if policy.Kind == "" {
			policy.Kind = "KongUpstreamPolicy"
		}
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": policy.APIVersion,
				"kind":       policy.Kind,
				"metadata":   map[string]interface{}{"name": policy.Metadata.Name, "namespace": targetNs},
				"spec":       policy.Spec,
			},
		}
		if err := s.applyUnstructured(ctx, kongUpstreamPolicyGVR, targetNs, obj); err != nil {
			result.Error = fmt.Sprintf("failed to apply KongUpstreamPolicy %s: %v", policy.Metadata.Name, err)
			return result
		}
	}

	// Create or update the migrated ingress in target namespace
	_, err = s.client.NetworkingV1().Ingresses(targetNs).Create(ctx, ingCopy, metav1.CreateOptions{})
	if err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			result.Error = fmt.Sprintf("failed to create ingress in %s: %v", targetNs, err)
			return result
		}
		existing, err := s.client.NetworkingV1().Ingresses(targetNs).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			result.Error = fmt.Sprintf("failed to get existing ingress in %s: %v", targetNs, err)
			return result
		}
		ingCopy.ResourceVersion = existing.ResourceVersion
		if _, err = s.client.NetworkingV1().Ingresses(targetNs).Update(ctx, ingCopy, metav1.UpdateOptions{}); err != nil {
			result.Error = fmt.Sprintf("failed to update ingress in %s: %v", targetNs, err)
			return result
		}
	}

	result.Success = true
	return result
}

// applyUnstructured creates the resource, or updates it if it already exists.
func (s *srv) applyUnstructured(ctx context.Context, gvr schema.GroupVersionResource, ns string, obj *unstructured.Unstructured) error {
	_, err := s.dynClient.Resource(gvr).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !k8serrors.IsAlreadyExists(err) {
		return err
	}
	existing, err := s.dynClient.Resource(gvr).Namespace(ns).Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		return err
	}
	obj.SetResourceVersion(existing.GetResourceVersion())
	_, err = s.dynClient.Resource(gvr).Namespace(ns).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

// ── YAML writers ──────────────────────────────────────────────────────────────

func writeOutputToWriter(w io.Writer, ingresses []networkingv1.Ingress, plugins []generator.KongPlugin, kongIngresses []generator.KongIngress, upstreamPolicies []generator.KongUpstreamPolicy) {
	for _, p := range plugins {
		if p.APIVersion == "" {
			p.APIVersion = "configuration.konghq.com/v1"
		}
		if p.Kind == "" {
			p.Kind = "KongPlugin"
		}
		if data, err := yaml.Marshal(p); err == nil {
			w.Write(data)
			io.WriteString(w, "---\n")
		}
	}
	for _, k := range kongIngresses {
		if k.APIVersion == "" {
			k.APIVersion = "configuration.konghq.com/v1"
		}
		if k.Kind == "" {
			k.Kind = "KongIngress"
		}
		if data, err := yaml.Marshal(k); err == nil {
			w.Write(data)
			io.WriteString(w, "---\n")
		}
	}
	for _, u := range upstreamPolicies {
		if u.APIVersion == "" {
			u.APIVersion = "configuration.konghq.com/v1beta1"
		}
		if u.Kind == "" {
			u.Kind = "KongUpstreamPolicy"
		}
		if data, err := yaml.Marshal(u); err == nil {
			w.Write(data)
			io.WriteString(w, "---\n")
		}
	}
	for _, ing := range ingresses {
		if data, err := yaml.Marshal(ing); err == nil {
			w.Write(data)
			io.WriteString(w, "---\n")
		}
	}
}

func writeGatewayAPIToWriter(w io.Writer, ingresses []networkingv1.Ingress, plugins []generator.KongPlugin) {
	const gatewayName, gatewayNamespace = "kong", "kong-system"
	for i := range ingresses {
		httpRoute := generator.ConvertToHTTPRoute(&ingresses[i], gatewayName, gatewayNamespace)
		ingressPlugins := findPluginsForIngress(&ingresses[i], plugins)
		for ruleIdx := range httpRoute.Spec.Rules {
			for _, plugin := range ingressPlugins {
				group := "configuration.konghq.com"
				httpRoute.Spec.Rules[ruleIdx].Filters = append(httpRoute.Spec.Rules[ruleIdx].Filters, generator.HTTPRouteFilter{
					Type: "ExtensionRef",
					ExtensionRef: &generator.LocalObjectReference{
						Group: &group,
						Kind:  "KongPlugin",
						Name:  plugin.Metadata.Name,
					},
				})
			}
		}
		if data, err := yaml.Marshal(httpRoute); err == nil {
			w.Write(data)
			io.WriteString(w, "---\n")
		}
	}
	for _, plugin := range plugins {
		if plugin.APIVersion == "" {
			plugin.APIVersion = "configuration.konghq.com/v1"
		}
		if plugin.Kind == "" {
			plugin.Kind = "KongPlugin"
		}
		if data, err := yaml.Marshal(plugin); err == nil {
			w.Write(data)
			io.WriteString(w, "---\n")
		}
	}
}

func findPluginsForIngress(ing *networkingv1.Ingress, plugins []generator.KongPlugin) []generator.KongPlugin {
	var result []generator.KongPlugin
	for _, plugin := range plugins {
		if len(plugin.Metadata.Name) > len(ing.Name) &&
			plugin.Metadata.Name[:len(ing.Name)] == ing.Name &&
			plugin.Metadata.Namespace == ing.Namespace {
			result = append(result, plugin)
		}
	}
	return result
}

// deepCopyIngress returns a deep copy with its own annotations map.
func deepCopyIngress(ing *networkingv1.Ingress) *networkingv1.Ingress {
	cp := *ing
	cp.Annotations = make(map[string]string, len(ing.Annotations))
	for k, v := range ing.Annotations {
		cp.Annotations[k] = v
	}
	return &cp
}
