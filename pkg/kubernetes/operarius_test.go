package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	operariusv1alpha1 "github.com/OpenFero/openfero/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
)

func TestInitOperariusInformerSyncsStore(t *testing.T) {
	sch := runtime.NewScheme()
	if err := operariusv1alpha1.AddToScheme(sch); err != nil {
		t.Fatalf("failed to add Operarius to scheme: %v", err)
	}
	if err := scheme.AddToScheme(sch); err != nil {
		t.Fatalf("failed to add k8s types to scheme: %v", err)
	}
	sch.AddKnownTypes(metav1.SchemeGroupVersion, &metav1.ListOptions{})

	listPayload, err := json.Marshal(&operariusv1alpha1.OperariusList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operariusv1alpha1.GroupVersion.String(),
			Kind:       "OperariusList",
		},
		ListMeta: metav1.ListMeta{ResourceVersion: "1"},
		Items: []operariusv1alpha1.Operarius{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-operarius",
					Namespace: "default",
				},
				Spec: operariusv1alpha1.OperariusSpec{
					AlertSelector: operariusv1alpha1.AlertSelector{
						AlertName: "HighCPU",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal OperariusList: %v", err)
	}

	var listCalled, watchCalled bool
	fakeClient := &fake.RESTClient{
		GroupVersion:         operariusv1alpha1.GroupVersion,
		NegotiatedSerializer: serializer.NewCodecFactory(sch).WithoutConversion(),
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			header := http.Header{"Content-Type": []string{"application/json"}}
			if req.URL.Query().Get("watch") == "true" {
				watchCalled = true
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     header,
					Body:       io.NopCloser(bytes.NewReader(nil)),
				}, nil
			}
			listCalled = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(bytes.NewReader(listPayload)),
			}, nil
		}),
	}

	client := &OperariusClient{
		restClient: fakeClient,
		namespace:  "default",
		scheme:     sch,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := client.InitOperariusInformer(ctx)
	if err != nil {
		t.Fatalf("InitOperariusInformer failed: %v", err)
	}

	if !listCalled {
		t.Error("expected list endpoint to be called")
	}
	if !watchCalled {
		t.Error("expected watch endpoint to be called")
	}

	items := store.List()
	if len(items) != 1 {
		t.Fatalf("expected 1 item in store, got %d", len(items))
	}

	listed, err := client.List()
	if err != nil {
		t.Fatalf("client.List() failed: %v", err)
	}
	if len(listed) != 1 || listed[0].Name != "test-operarius" {
		t.Fatalf("unexpected listed items: %+v", listed)
	}
}
