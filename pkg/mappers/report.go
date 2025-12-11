package mappers

import (
	"fmt"
	"net/url"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
)

// ReportUnmigratedAnnotations scans the provided Ingresses for any remaining
// nginx.ingress.kubernetes.io/* annotations that were not handled (removed) by the mappers.
// It prints a summary and instructions for the user to report them.
func ReportUnmigratedAnnotations(ing *networkingv1.Ingress) {
	unmigrated := make(map[string]int)

	for key := range ing.Annotations {
		if strings.HasPrefix(key, "nginx.ingress.kubernetes.io/") {
			unmigrated[key]++
		}
	}

	if len(unmigrated) == 0 {
		fmt.Println("\n✨ Success! All NGINX annotations were migrated or handled.")
		return
	}

	fmt.Println("\n⚠️  Unmigrated Annotations Detected")
	fmt.Println("The following NGINX annotations were found but not migrated (or explicitly ignored):")
	fmt.Println("--------------------------------------------------------------------------------")
	for key, count := range unmigrated {
		fmt.Printf("- %s (found in %d Ingresses)\n", key, count)
	}
	fmt.Println("--------------------------------------------------------------------------------")

	// Construct GitHub Issue Link
	baseURL := "https://github.com/Kong/nginx-kong-migrator/issues/new"
	title := "Unmigrated Annotations Report"
	body := "I run the migration tool and found the following unmigrated annotations:\n\n"
	for key := range unmigrated {
		body += fmt.Sprintf("- %s\n", key)
	}
	body += "\nPlease add support for these!"

	// URL Encode parameters
	params := url.Values{}
	params.Add("title", title)
	params.Add("body", body)
	params.Add("labels", "enhancement,generated-report")

	issueURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	fmt.Println("\n📞 Help us improve! Please report these missing features by opening a GitHub issue:")
	fmt.Printf("\n%s\n\n", issueURL)
}
