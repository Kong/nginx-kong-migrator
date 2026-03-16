package ui

import (
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// newTestSrv creates a *srv backed by fake Kubernetes clients for unit testing.
func newTestSrv(version string, objs ...runtime.Object) *srv {
	return &srv{
		namespace: "",
		version:   version,
		client:    kubefake.NewSimpleClientset(objs...),
		dynClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
	}
}
