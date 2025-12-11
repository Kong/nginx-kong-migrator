package parser

import (
	"os"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"
)

// ParseIngress reads a YAML file and extracts Ingress objects.
// Note: This is a simplified parser. Real-world usage should handle multi-document YAMLs properly.
func ParseIngress(filename string) ([]networkingv1.Ingress, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var ingresses []networkingv1.Ingress
	
	// Split by "---" for multi-doc support
	docs := strings.Split(string(data), "\n---")
	
	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		
		var ing networkingv1.Ingress
		if err := yaml.Unmarshal([]byte(doc), &ing); err != nil {
			// In a real tool we might check if Kind=Ingress before unmarshalling errors, 
			// but for this MVP we assume input is Ingresses.
			continue 
		}
		
		if ing.Kind == "Ingress" && ing.APIVersion == "networking.k8s.io/v1" {
			ingresses = append(ingresses, ing)
		}
	}

	return ingresses, nil
}
