package mappers

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeIngress(name, ns string, annotations map[string]string) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
		},
	}
}

func makeIngressWithRules(name, ns string, annotations map[string]string, rules []networkingv1.IngressRule) *networkingv1.Ingress {
	ing := makeIngress(name, ns, annotations)
	ing.Spec.Rules = rules
	return ing
}
