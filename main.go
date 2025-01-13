package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type DiunWebhook struct {
	Metadata DiunMetadata
}

type DiunMetadata struct {
	PodName string `json:"pod_name"`
}

func main() {
	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		var diunWebhook DiunWebhook
		err := json.NewDecoder(r.Body).Decode(&diunWebhook)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		restartPod(diunWebhook.Metadata.PodName)
	})

	http.ListenAndServe(":8080", nil)
}

func restartPod(podName string) {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	log.Printf("Updating %s", podName)
	pod, err := clientset.CoreV1().Pods("").Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		log.Println(fmt.Errorf("Couldn't fetch Pod", err))
		return
	}
	if pod.ObjectMeta.OwnerReferences[0].Kind != "ReplicaSet" {
		log.Println("Not controlled by ReplicaSet")
		return
	}
	rs, err := clientset.AppsV1().ReplicaSets("").Get(context.TODO(), pod.ObjectMeta.OwnerReferences[0].Name, metav1.GetOptions{})
	if rs != nil {
		log.Println(fmt.Errorf("Couldn't fetch ReplicaSet", err))
		return
	}
	if rs.ObjectMeta.OwnerReferences[0].Kind != "Deployment" {
		log.Println("Not controlled by Deployment")
		return
	}
	data := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format("20060102150405"))
	_, err = clientset.AppsV1().Deployments("").Patch(context.TODO(), rs.ObjectMeta.OwnerReferences[0].Name, types.StrategicMergePatchType, []byte(data), v1.PatchOptions{})
	if err != nil {
		log.Println(fmt.Errorf("Failed to restart deployment %s", &rs.ObjectMeta.OwnerReferences[0].Name, err))
		return
	}
	log.Printf("Updated %s successfully", podName)
}
