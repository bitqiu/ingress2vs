# ingress 2 vs
> Convert Ingress to Istio VirtualService.

## Usage

```bash
kubectl get -n default ingress

go run main.go -n default -i ingress-nginx

# Check the converted configuration
kubectl get -n default virtualservice

```