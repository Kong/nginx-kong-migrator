package generator

import (
	"os"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"
)

// KongPlugin represents a KongPlugin Custom Resource (CRD).
// We define a struct here to avoid needing the full Kong implementation for this MVP.
type ObjectMeta struct {
	Name        string            `json:"name" yaml:"name"`
	Namespace   string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

// KongUpstreamPolicy represents the new Upstream configuration resource
type KongUpstreamPolicy struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   ObjectMeta             `json:"metadata"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
}

type KongPlugin struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   ObjectMeta             `json:"metadata"`
	Plugin     string                 `json:"plugin"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

type KongIngress struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   ObjectMeta             `json:"metadata"`
	Upstream   map[string]interface{} `json:"upstream,omitempty"`
	Proxy      map[string]interface{} `json:"proxy,omitempty"`
	Route      map[string]interface{} `json:"route,omitempty"`
}

func WriteOutput(ingresses []networkingv1.Ingress, plugins []KongPlugin, kongIngresses []KongIngress, upstreamPolicies []KongUpstreamPolicy, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Combine plugins and ingresses into a single output list
	var output []interface{}
	for _, p := range plugins {
		if p.APIVersion == "" {
			p.APIVersion = "configuration.konghq.com/v1"
		}
		if p.Kind == "" {
			p.Kind = "KongPlugin"
		}
		output = append(output, p)
	}
	// Deprecated KongIngress (keep for now if any remaining, but aim to migrate)
	for _, k := range kongIngresses {
		if k.APIVersion == "" {
			k.APIVersion = "configuration.konghq.com/v1"
		}
		if k.Kind == "" {
			k.Kind = "KongIngress"
		}
		output = append(output, k)
	}
	// New KongUpstreamPolicy
	for _, u := range upstreamPolicies {
		if u.APIVersion == "" {
			u.APIVersion = "configuration.konghq.com/v1beta1"
		}
		if u.Kind == "" {
			u.Kind = "KongUpstreamPolicy"
		}
		output = append(output, u)
	}

	for _, ing := range ingresses {
		output = append(output, ing)
	}

	for _, item := range output {
		data, err := yaml.Marshal(item)
		if err != nil {
			return err
		}
		f.Write(data)
		f.WriteString("---\n")
	}

	return nil
}

// WriteGatewayAPIOutput writes Gateway API resources (HTTPRoute + KongPlugins) to a file
func WriteGatewayAPIOutput(ingresses []networkingv1.Ingress, plugins []KongPlugin, filename, gatewayName, gatewayNamespace string) error {
	var resources []interface{}

	// Convert each Ingress to HTTPRoute
	for i := range ingresses {
		httpRoute := ConvertToHTTPRoute(&ingresses[i], gatewayName, gatewayNamespace)

		// Attach plugins to HTTPRoute via ExtensionRef filters
		// Find plugins for this ingress
		ingressPlugins := findPluginsForIngress(&ingresses[i], plugins)
		if len(ingressPlugins) > 0 {
			// Add ExtensionRef filters for each plugin to each rule
			for ruleIdx := range httpRoute.Spec.Rules {
				for _, plugin := range ingressPlugins {
					group := "configuration.konghq.com"
					filter := HTTPRouteFilter{
						Type: "ExtensionRef",
						ExtensionRef: &LocalObjectReference{
							Group: &group,
							Kind:  "KongPlugin",
							Name:  plugin.Metadata.Name,
						},
					}
					httpRoute.Spec.Rules[ruleIdx].Filters = append(httpRoute.Spec.Rules[ruleIdx].Filters, filter)
				}
			}
		}

		resources = append(resources, httpRoute)
	}

	// Add KongPlugins with proper apiVersion and kind
	for _, plugin := range plugins {
		// Ensure apiVersion and kind are set
		if plugin.APIVersion == "" {
			plugin.APIVersion = "configuration.konghq.com/v1"
		}
		if plugin.Kind == "" {
			plugin.Kind = "KongPlugin"
		}
		resources = append(resources, plugin)
	}

	// Write to file or stdout
	if filename == "" {
		for _, resource := range resources {
			data, err := yaml.Marshal(resource)
			if err != nil {
				return err
			}
			os.Stdout.Write(data)
			os.Stdout.WriteString("---\n")
		}
		return nil
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write each resource
	for _, resource := range resources {
		data, err := yaml.Marshal(resource)
		if err != nil {
			return err
		}
		_, _ = f.Write(data)
		_, _ = f.WriteString("---\n")
	}

	return nil
}

// findPluginsForIngress finds all plugins that belong to a specific Ingress
func findPluginsForIngress(ing *networkingv1.Ingress, plugins []KongPlugin) []KongPlugin {
	var result []KongPlugin
	ingressName := ing.Name

	for _, plugin := range plugins {
		// Check if plugin name starts with ingress name (our naming convention)
		if len(plugin.Metadata.Name) > len(ingressName) &&
			plugin.Metadata.Name[:len(ingressName)] == ingressName &&
			plugin.Metadata.Namespace == ing.Namespace {
			result = append(result, plugin)
		}
	}

	return result
}
