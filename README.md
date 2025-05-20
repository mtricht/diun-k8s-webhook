# diun-k8s-webhook

When a new image is available and a webhook payload from [diun](https://github.com/crazy-max/diun) is received with the corresponding pod name, this package automatically finds the deployment that is using that docker image and restarts it.

Requires view and edit cluster role permissions:
```
kubectl create clusterrolebinding default-view --clusterrole=view --serviceaccount=default:default
kubectl create clusterrolebinding default-view --clusterrole=edit --serviceaccount=default:default
```

Example kubernetes manifest YAML files:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    io.kompose.service: diun-k8s-webhook
  name: diun-k8s-webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      io.kompose.service: diun-k8s-webhook
  template:
    metadata:
      labels:
        io.kompose.service: diun-k8s-webhook
    spec:
      containers:
        - name: diun-k8s-webhook
          image: ghcr.io/mtricht/diun-k8s-webhook:latest
          ports:
            - containerPort: 8080
              protocol: TCP
      restartPolicy: Always
---
apiVersion: v1
kind: Service
metadata:
  labels:
    io.kompose.service: diun-k8s-webhook
  name: diun-k8s-webhook
spec:
  ports:
    - name: "8080"
      port: 8080
      targetPort: 8080
  selector:
    io.kompose.service: diun-k8s-webhook
```

Add the webhook URL to diun:
```
DIUN_NOTIF_WEBHOOK_ENDPOINT=http://diun-k8s-webhook.default:8080/webhook
DIUN_NOTIF_WEBHOOK_METHOD=POST
```
