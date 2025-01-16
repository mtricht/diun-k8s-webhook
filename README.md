# diun-k8s-webhook

Requires view and edit cluster role permissions:
```
kubectl create clusterrolebinding default-view --clusterrole=view --serviceaccount=default:default
kubectl create clusterrolebinding default-view --clusterrole=edit --serviceaccount=default:default
```