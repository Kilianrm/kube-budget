kind create cluster --name cluster-to-be-monitorized --config kind-cluster.yaml
kubectl cluster-info
kubectl get nodes
kubectl apply -f cluster-resources.yaml
kubectl get all -n monitoring-demo