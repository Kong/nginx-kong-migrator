package main

import (
	"flag"
	"fmt"
	"log"

	"nginx-kong-migrator/pkg/generator"
	"nginx-kong-migrator/pkg/mappers"
	"nginx-kong-migrator/pkg/parser"
)

func main() {
	inputFile := flag.String("f", "", "Input file containing NGINX Ingress manifests")
	outputFile := flag.String("o", "migrated-ingress.yaml", "Output file for Kong manifests")
	ingressClass := flag.String("ingress-class", "", "Target Ingress Class Name (e.g. kong)")
	flag.Parse()

	if *inputFile == "" {
		log.Fatal("Input file is required (-f)")
	}

	// 1. Parse Input
	ingresses, err := parser.ParseIngress(*inputFile)
	if err != nil {
		log.Fatalf("Failed to parse input: %v", err)
	}

	// 2. Map and Generate
	var allKongPlugins []generator.KongPlugin
	var allKongIngresses []generator.KongIngress
	var allKongUpstreamPolicies []generator.KongUpstreamPolicy

	for i := range ingresses {
		// Update Ingress Class if specified
		if *ingressClass != "" {
			ingresses[i].Spec.IngressClassName = ingressClass
		}

		// 3. Transform
		var currentPlugins []generator.KongPlugin
		var currentKongIngresses []generator.KongIngress
		var currentUpstreamPolicies []generator.KongUpstreamPolicy

		mappers.Apply(&ingresses[i], &currentPlugins, &currentKongIngresses, &currentUpstreamPolicies)
		mappers.ReportUnmigratedAnnotations(&ingresses[i]) // Report unmigrated annotations for the current ingress

		allKongPlugins = append(allKongPlugins, currentPlugins...)
		allKongIngresses = append(allKongIngresses, currentKongIngresses...)
		allKongUpstreamPolicies = append(allKongUpstreamPolicies, currentUpstreamPolicies...)
	}

	// 4. Write Output
	// Signature: func WriteOutput(ingress *networkingv1.Ingress, plugins []KongPlugin, kongIngresses []KongIngress, upstreamPolicies []KongUpstreamPolicy, filename string) error
	// CAUTION: Generator expects single ingress pointer but we have a slice.
	// We need to update generator to accept slice or loop here.

	// Let's update generator.go to accept slice of Ingresses.
	if err := generator.WriteOutput(ingresses, allKongPlugins, allKongIngresses, allKongUpstreamPolicies, *outputFile); err != nil {
		log.Fatalf("Error writing output: %v", err)
	}

	fmt.Printf("Migration complete. Output written to %s\n", *outputFile)
}
