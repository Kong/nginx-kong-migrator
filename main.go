package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"nginx-kong-migrator/pkg/generator"
	"nginx-kong-migrator/pkg/mappers"
	"nginx-kong-migrator/pkg/parser"
	"nginx-kong-migrator/pkg/ui"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(Version)
		return
	}

	// ui subcommand — spins up the local dashboard server
	if len(os.Args) > 1 && os.Args[1] == "ui" {
		uiFlags := flag.NewFlagSet("ui", flag.ExitOnError)
		port := uiFlags.Int("port", 8080, "Port to listen on")
		namespace := uiFlags.String("namespace", "", "Kubernetes namespace to watch (empty = all namespaces)")
		kubeconfig := uiFlags.String("kubeconfig", "", "Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
		uiFlags.Parse(os.Args[2:])
		if err := ui.Start(*port, *namespace, *kubeconfig, Version); err != nil {
			log.Fatalf("ui server error: %v", err)
		}
		return
	}

	var (
		inputFile    = flag.String("input", "", "Path to NGINX Ingress YAML file")
		outputFile   = flag.String("output", "", "Path to output Kong YAML file (default: stdout)")
		outputFormat = flag.String("output-format", "kong-ingress", "Output format: kong-ingress, gateway-api, or both")
	)
	ingressClass := flag.String("ingress-class", "", "Target Ingress Class Name (e.g. kong)")
	flag.Parse()

	// Validate output format
	validFormats := map[string]bool{
		"kong-ingress": true,
		"gateway-api":  true,
		"both":         true,
	}
	if !validFormats[*outputFormat] {
		log.Fatalf("Invalid output format: %s. Must be kong-ingress, gateway-api, or both", *outputFormat)
	}

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

	// 4. Write Output based on format
	outputFilename := *outputFile
	if outputFilename == "" {
		outputFilename = "migrated-output.yaml"
	}

	switch *outputFormat {
	case "kong-ingress":
		if err := generator.WriteOutput(ingresses, allKongPlugins, allKongIngresses, allKongUpstreamPolicies, outputFilename); err != nil {
			log.Fatalf("Error writing output: %v", err)
		}
		fmt.Printf("Migration complete. Output written to %s\n", outputFilename)

	case "gateway-api":
		// Use default gateway name/namespace
		gatewayName := "kong"
		gatewayNamespace := "kong-system"

		if err := generator.WriteGatewayAPIOutput(ingresses, allKongPlugins, outputFilename, gatewayName, gatewayNamespace); err != nil {
			log.Fatalf("Error writing Gateway API output: %v", err)
		}
		fmt.Printf("Migration complete. Gateway API output written to %s\n", outputFilename)

	case "both":
		// Write Kong Ingress format
		kongFile := "migrated-kong-ingress.yaml"
		if err := generator.WriteOutput(ingresses, allKongPlugins, allKongIngresses, allKongUpstreamPolicies, kongFile); err != nil {
			log.Fatalf("Error writing Kong Ingress output: %v", err)
		}

		// Write Gateway API format
		gatewayFile := "migrated-gateway-api.yaml"
		gatewayName := "kong"
		gatewayNamespace := "kong-system"

		if err := generator.WriteGatewayAPIOutput(ingresses, allKongPlugins, gatewayFile, gatewayName, gatewayNamespace); err != nil {
			log.Fatalf("Error writing Gateway API output: %v", err)
		}
		fmt.Printf("Migration complete.\n")
		fmt.Printf("  Kong Ingress: %s\n", kongFile)
		fmt.Printf("  Gateway API: %s\n", gatewayFile)
	}
}
