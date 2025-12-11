package generator

import (
	"os"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"
)

// KongPlugin represents a KongPlugin Custom Resource (CRD).
// We define a struct here to avoid needing the full Kong implementation for this MVP.
type ObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
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
