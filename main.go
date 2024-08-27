package main

import (
	"context"
	"flag"
	"fmt"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istioNetworking "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioClient "istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"log"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
	namespace := flag.String("namespace", "", "Kubernetes namespace")
	ingressName := flag.String("ingressname", "", "Name of the Ingress resource")
	flag.StringVar(namespace, "n", "default", "Kubernetes namespace (shorthand)")
	flag.StringVar(ingressName, "i", "", "Name of the Ingress resource (shorthand)")

	flag.Parse()

	if *namespace == "" || *ingressName == "" {
		log.Fatalf("Usage: %s --namespace <namespace> --ingressname <ingressname>", os.Args[0])
	}

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("Error getting Kubernetes config: %v", err)
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	istioClient, err := istioClient.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error creating Istio client: %v", err)
	}

	ingress, err := k8sClient.NetworkingV1().Ingresses(*namespace).Get(context.Background(), *ingressName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Error getting Ingress resource: %v", err)
	}

	virtualService := &istioNetworking.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
		},
		Spec: networkingv1alpha3.VirtualService{
			Hosts:    []string{ingress.Spec.Rules[0].Host},
			Gateways: []string{fmt.Sprintf("%s/ingressgateway", ingress.Namespace)},
		},
	}

	httpToHttpsRedirect := &networkingv1alpha3.HTTPRoute{
		Match: []*networkingv1alpha3.HTTPMatchRequest{
			{
				Port: 80,
			},
		},
		Name: "http-to-https",
		Redirect: &networkingv1alpha3.HTTPRedirect{
			Scheme: "https",
		},
	}

	virtualService.Spec.Http = append(virtualService.Spec.Http, httpToHttpsRedirect)

	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			httpRoute := &networkingv1alpha3.HTTPRoute{
				Match: []*networkingv1alpha3.HTTPMatchRequest{
					{
						Port: 443,
					},
				},
				Name: "pos-bff-service",
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					{
						Destination: &networkingv1alpha3.Destination{
							Host: fmt.Sprintf("%s", path.Backend.Service.Name),
							Port: &networkingv1alpha3.PortSelector{
								Number: uint32(path.Backend.Service.Port.Number),
							},
						},
					},
				},
			}

			virtualService.Spec.Http = append(virtualService.Spec.Http, httpRoute)
		}
	}

	_, err = istioClient.NetworkingV1alpha3().VirtualServices(*namespace).Create(context.Background(), virtualService, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating VirtualService resource: %v", err)
	}

	fmt.Printf("Successfully created VirtualService %s in namespace %s\n", virtualService.Name, virtualService.Namespace)
}
