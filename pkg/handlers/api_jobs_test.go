package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	operariusv1alpha1 "github.com/OpenFero/openfero/api/v1alpha1"
	"github.com/OpenFero/openfero/pkg/models"
	"github.com/OpenFero/openfero/pkg/services"
)

func TestJobsAPIHandler_IncludesSelectorAndFlags(t *testing.T) {
	disabled := false
	stub := &stubOperariusClient{
		namespace: "openfero",
		operarii: []operariusv1alpha1.Operarius{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-restart-dryrun",
					Namespace: "openfero",
				},
				Spec: operariusv1alpha1.OperariusSpec{
					AlertSelector: operariusv1alpha1.AlertSelector{
						AlertName: "PodCrashLooping",
						Status:    "firing",
						Labels: map[string]string{
							"severity": "critical",
						},
					},
					Priority: 10,
					DryRun:   true,
					Enabled:  &disabled,
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RestartPolicy: corev1.RestartPolicyNever,
									Containers: []corev1.Container{
										{Name: "remediate", Image: "ghcr.io/openfero/remediate:latest"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	fakeKube := fake.NewSimpleClientset()
	srv := &Server{
		OperariusService: services.NewOperariusServiceWithClient(fakeKube, stub),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	rec := httptest.NewRecorder()
	srv.JobsAPIHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got []models.JobInfo
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)

	item := got[0]
	assert.Equal(t, "pod-restart-dryrun", item.OperariusName)
	assert.Equal(t, "PodCrashLooping", item.JobName)
	assert.Equal(t, "firing", item.AlertStatus)
	assert.Equal(t, map[string]string{"severity": "critical"}, item.AlertLabels)
	assert.True(t, item.DryRun)
	require.NotNil(t, item.Enabled)
	assert.False(t, *item.Enabled)
	assert.Equal(t, int32(10), item.Priority)
	assert.Equal(t, "ghcr.io/openfero/remediate:latest", item.Image)
}
