package kubernetes

import (
	"context"
	"os"
	"path/filepath"
	"time"

	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/metadata"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// Client represents a Kubernetes client with necessary stores and configuration
type Client struct {
	Clientset     kubernetes.Clientset
	LabelSelector *metav1.LabelSelector
}

// InitKubeClient initializes a Kubernetes client using in-cluster or kubeconfig
func InitKubeClient(kubeconfig *string) *kubernetes.Clientset {
	var config *rest.Config
	var err error

	// Try in-cluster config first
	config, err = rest.InClusterConfig()
	if err != nil {
		log.Debug("In-cluster configuration not available, trying kubeconfig file", "error", err)

		kubeconfigPath := *kubeconfig
		if kubeconfigPath == "" {
			if home, homeErr := os.UserHomeDir(); homeErr == nil {
				defaultPath := filepath.Join(home, ".kube", "config")
				if _, statErr := os.Stat(defaultPath); statErr == nil {
					kubeconfigPath = defaultPath
				}
			}
		}

		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			log.Fatal("Could not create k8s configuration", "error", err)
		}
		log.Info("Using kubeconfig file for cluster access", "kubeconfigPath", kubeconfigPath)
	} else {
		log.Info("Using in-cluster configuration")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Could not create k8s client", "error", err)
	}

	return clientset
}

// GetCurrentNamespace determines the current namespace
func GetCurrentNamespace() (string, error) {
	// Check if running in-cluster
	_, err := rest.InClusterConfig()

	if err != nil {
		// Out-of-cluster
		log.Debug("Using out of cluster configuration")
		// Extract namespace from client config
		namespace, _, err := clientcmd.DefaultClientConfig.Namespace()
		return namespace, err
	} else {
		// In-cluster
		log.Debug("Using in cluster configuration")
		// Read namespace from mounted secrets
		defaultNamespaceLocation := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
		if _, statErr := os.Stat(defaultNamespaceLocation); os.IsNotExist(statErr) {
			return "", statErr
		}
		namespaceDat, readErr := os.ReadFile(defaultNamespaceLocation)
		if readErr != nil {
			return "", readErr
		}
		return string(namespaceDat), nil
	}
}

// InitJobInformer initializes a Job informer
func InitJobInformer(clientset *kubernetes.Clientset, jobDestinationNamespace string, labelSelector *metav1.LabelSelector, updateFunc func(oldJob, newJob *batchv1.Job)) cache.Store {
	// Create informer factory
	jobFactory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		time.Hour*1,
		informers.WithNamespace(jobDestinationNamespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = metav1.FormatLabelSelector(labelSelector)
		}),
	)

	log.Debug("Initializing Job informer",
		"namespace", jobDestinationNamespace,
		"labelSelector", metav1.FormatLabelSelector(labelSelector))

	// Get Job informer
	jobInformer := jobFactory.Batch().V1().Jobs().Informer()

	// Add event handlers
	if _, err := jobInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			job := obj.(*batchv1.Job)
			log.Debug("Job added", "job", job.Name, "namespace", job.Namespace)
			metadata.JobsCreatedTotal.Inc()
			if updateFunc != nil {
				updateFunc(nil, job)
			}
		},
		UpdateFunc: func(old, new any) {
			oldJob := old.(*batchv1.Job)
			newJob := new.(*batchv1.Job)
			if newJob.Status.Succeeded > 0 && oldJob.Status.Succeeded == 0 {
				log.Debug("Job completed successfully", "job", newJob.Name, "namespace", newJob.Namespace)
				metadata.JobsSucceededTotal.Inc()
			}
			if newJob.Status.Failed > 0 && oldJob.Status.Failed == 0 {
				log.Debug("Job failed", "job", newJob.Name, "namespace", newJob.Namespace)
				metadata.JobsFailedTotal.Inc()
			}
			if updateFunc != nil {
				updateFunc(oldJob, newJob)
			}
		},
		DeleteFunc: func(obj any) {
			job := obj.(*batchv1.Job)
			log.Debug("Job deleted", "job", job.Name, "namespace", job.Namespace)
		},
	}); err != nil {
		log.Fatal("Failed to add Job event handler", "error", err)
	}

	// Start informer
	go jobFactory.Start(context.Background().Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(context.Background().Done(), jobInformer.HasSynced) {
		log.Fatal("Failed to sync Job cache", "namespace", jobDestinationNamespace)
	}
	log.Info("Job cache synced", "namespace", jobDestinationNamespace)

	return jobInformer.GetStore()
}
