package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type DiunWebhook struct {
	Digest   string
	Metadata DiunMetadata
}

type DiunMetadata struct {
	ContainerName string `json:"ctn_name"`
	PodName       string `json:"pod_name"`
	PodNamespace  string `json:"pod_namespace"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		var diunWebhook DiunWebhook
		err := json.NewDecoder(r.Body).Decode(&diunWebhook)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		restartPod(diunWebhook)
	})
	
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
}

func restartPod(webhook DiunWebhook) {
	metadata := webhook.Metadata

	config, err := rest.InClusterConfig()
	if err != nil {
		var kubeconfig string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Printf("Error creating config from kubeconfig: %v", err)
			return
		}
	}
	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
	}
	log.Printf("updating %s", metadata.PodName)
	pod, err := clientset.CoreV1().Pods(metadata.PodNamespace).Get(context.TODO(), metadata.PodName, metav1.GetOptions{})
	if err != nil {
		log.Println("couldn't fetch Pod", err)
		return
	}
	if pod.ObjectMeta.OwnerReferences[0].Kind != "ReplicaSet" {
		log.Println("not controlled by ReplicaSet")
		return
	}
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == metadata.ContainerName && strings.Contains(containerStatus.ImageID, webhook.Digest) {
			log.Println("skipping pod as it is already up to date")
			return
		}
	}
	rs, err := clientset.AppsV1().ReplicaSets(metadata.PodNamespace).Get(context.TODO(), pod.ObjectMeta.OwnerReferences[0].Name, metav1.GetOptions{})
	if err != nil {
		log.Println("couldn't fetch ReplicaSet", err)
		return
	}
	if rs.ObjectMeta.OwnerReferences[0].Kind != "Deployment" {
		log.Println("not controlled by Deployment")
		return
	}
	data := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format("20060102150405"))
	_, err = clientset.AppsV1().Deployments(metadata.PodNamespace).Patch(context.TODO(), rs.ObjectMeta.OwnerReferences[0].Name, types.StrategicMergePatchType, []byte(data), v1.PatchOptions{})
	if err != nil {
		log.Println(fmt.Errorf("failed to restart deployment %p %w", &rs.ObjectMeta.OwnerReferences[0].Name, err))
		return
	}
	log.Printf("updated %s successfully", metadata.PodName)
}
