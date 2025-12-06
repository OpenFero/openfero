package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	operariusv1alpha1 "github.com/OpenFero/openfero/api/v1alpha1"
	log "github.com/OpenFero/openfero/pkg/logging"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// OperariusClient represents a client for Operarius CRDs
type OperariusClient struct {
	client    ctrlclient.Client
	namespace string
	store     cache.Store
	storeMu   sync.RWMutex
	informer  cache.SharedIndexInformer
	scheme    *runtime.Scheme
}

// NewOperariusClient creates a new client for Operarius CRDs
func NewOperariusClient(kubeconfig *string, namespace string) (*OperariusClient, error) {
	config, err := getRestConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %w", err)
	}

	// Create scheme and register Operarius types
	scheme := runtime.NewScheme()
	if err := operariusv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add Operarius to scheme: %w", err)
	}

	// Create controller-runtime client
	client, err := ctrlclient.New(config, ctrlclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &OperariusClient{
		client:    client,
		namespace: namespace,
		scheme:    scheme,
	}, nil
}

// getRestConfig returns the Kubernetes REST config
func getRestConfig(kubeconfig *string) (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig
		if kubeconfig != nil && *kubeconfig != "" {
			config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("no kubeconfig available: %w", err)
		}
	}
	return config, nil
}

// InitOperariusInformer initializes the Operarius informer and returns the store
func (c *OperariusClient) InitOperariusInformer(ctx context.Context, restConfig *rest.Config, kubeconfigPath *string) (cache.Store, error) {
	// Get REST config if not provided
	if restConfig == nil {
		var err error
		restConfig, err = getRestConfig(kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get REST config: %w", err)
		}
	}

	// Create list/watch functions using controller-runtime client
	listFunc := func(options metav1.ListOptions) (runtime.Object, error) {
		list := &operariusv1alpha1.OperariusList{}
		listOpts := []ctrlclient.ListOption{
			ctrlclient.InNamespace(c.namespace),
		}
		if err := c.client.List(ctx, list, listOpts...); err != nil {
			return nil, err
		}
		return list, nil
	}

	watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		// Watch not implemented - using resync for cache updates
		// Note: The GetOperariiForNamespace function reads directly from API
		// to ensure newly created Operarii are immediately available
		return nil, fmt.Errorf("watch not implemented - using resync")
	}

	// Create informer with list (watch uses resync)
	c.informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  listFunc,
			WatchFunc: watchFunc,
		},
		&operariusv1alpha1.Operarius{},
		time.Minute*1, // Resync period for cache updates
		cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		},
	)

	// Add event handlers
	_, handlerErr := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			operarius, ok := obj.(*operariusv1alpha1.Operarius)
			if ok {
				log.Debug("Operarius added to store",
					zap.String("name", operarius.Name),
					zap.String("namespace", operarius.Namespace),
					zap.String("alertname", operarius.Spec.AlertSelector.AlertName))
			}
		},
		UpdateFunc: func(old, new interface{}) {
			operarius, ok := new.(*operariusv1alpha1.Operarius)
			if ok {
				log.Debug("Operarius updated in store",
					zap.String("name", operarius.Name),
					zap.String("namespace", operarius.Namespace))
			}
		},
		DeleteFunc: func(obj interface{}) {
			operarius, ok := obj.(*operariusv1alpha1.Operarius)
			if ok {
				log.Debug("Operarius removed from store",
					zap.String("name", operarius.Name),
					zap.String("namespace", operarius.Namespace))
			}
		},
	})
	if handlerErr != nil {
		return nil, fmt.Errorf("failed to add event handler: %w", handlerErr)
	}

	// Start informer in background
	go c.informer.Run(ctx.Done())

	// Wait for cache sync
	log.Info("Waiting for Operarius cache to sync",
		zap.String("namespace", c.namespace))

	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if !cache.WaitForCacheSync(syncCtx.Done(), c.informer.HasSynced) {
		return nil, fmt.Errorf("failed to sync Operarius cache within timeout")
	}

	c.store = c.informer.GetStore()
	log.Info("Operarius cache synced",
		zap.String("namespace", c.namespace),
		zap.Int("count", len(c.store.List())))

	return c.store, nil
}

// GetStore returns the informer store
func (c *OperariusClient) GetStore() cache.Store {
	return c.store
}

// List returns all Operarius resources from the cache
func (c *OperariusClient) List() ([]operariusv1alpha1.Operarius, error) {
	if c.store == nil {
		return nil, fmt.Errorf("store not initialized, call InitOperariusInformer first")
	}

	objects := c.store.List()
	operarii := make([]operariusv1alpha1.Operarius, 0, len(objects))

	for _, obj := range objects {
		operarius, ok := obj.(*operariusv1alpha1.Operarius)
		if ok {
			operarii = append(operarii, *operarius)
		}
	}

	return operarii, nil
}

// ListFromAPI fetches Operarius resources directly from the API (not cache)
func (c *OperariusClient) ListFromAPI(ctx context.Context) ([]operariusv1alpha1.Operarius, error) {
	list := &operariusv1alpha1.OperariusList{}
	listOpts := []ctrlclient.ListOption{
		ctrlclient.InNamespace(c.namespace),
	}
	if err := c.client.List(ctx, list, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list Operarii: %w", err)
	}
	return list.Items, nil
}

// Get returns a specific Operarius by name from the cache
func (c *OperariusClient) Get(name string) (*operariusv1alpha1.Operarius, error) {
	if c.store == nil {
		return nil, fmt.Errorf("store not initialized, call InitOperariusInformer first")
	}

	key := c.namespace + "/" + name
	obj, exists, err := c.store.GetByKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get Operarius: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("Operarius %s not found", name)
	}

	operarius, ok := obj.(*operariusv1alpha1.Operarius)
	if !ok {
		return nil, fmt.Errorf("object is not an Operarius")
	}

	return operarius, nil
}

// UpdateStatus updates the status of an Operarius resource
func (c *OperariusClient) UpdateStatus(ctx context.Context, operarius *operariusv1alpha1.Operarius) error {
	if err := c.client.Status().Update(ctx, operarius); err != nil {
		return fmt.Errorf("failed to update Operarius status: %w", err)
	}

	log.Debug("Updated Operarius status",
		zap.String("name", operarius.Name),
		zap.Int32("executionCount", operarius.Status.ExecutionCount))

	return nil
}

// GetNamespace returns the namespace this client is configured for
func (c *OperariusClient) GetNamespace() string {
	return c.namespace
}
