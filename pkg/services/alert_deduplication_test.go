package services

import (
	"testing"

	"github.com/OpenFero/openfero/pkg/kubernetes"
	log "github.com/OpenFero/openfero/pkg/logging"
	"github.com/OpenFero/openfero/pkg/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	// Initialize logger for tests
	cfg := zap.NewDevelopmentConfig()
	err := log.SetConfig(cfg)
	if err != nil {
		panic(err)
	}
}

// MockJobStore implements cache.Store for testing
type MockJobStore struct {
	jobs map[string]*batchv1.Job
}

func NewMockJobStore() *MockJobStore {
	return &MockJobStore{
		jobs: make(map[string]*batchv1.Job),
	}
}

func (m *MockJobStore) Add(obj interface{}) error {
	job := obj.(*batchv1.Job)
	m.jobs[job.Namespace+"/"+job.Name] = job
	return nil
}

func (m *MockJobStore) Update(obj interface{}) error {
	return m.Add(obj)
}

func (m *MockJobStore) Delete(obj interface{}) error {
	job := obj.(*batchv1.Job)
	delete(m.jobs, job.Namespace+"/"+job.Name)
	return nil
}

func (m *MockJobStore) List() []interface{} {
	var result []interface{}
	for _, job := range m.jobs {
		result = append(result, job)
	}
	return result
}

func (m *MockJobStore) ListKeys() []string {
	var keys []string
	for key := range m.jobs {
		keys = append(keys, key)
	}
	return keys
}

func (m *MockJobStore) Get(obj interface{}) (interface{}, bool, error) {
	job := obj.(*batchv1.Job)
	key := job.Namespace + "/" + job.Name
	result, exists := m.jobs[key]
	return result, exists, nil
}

func (m *MockJobStore) GetByKey(key string) (interface{}, bool, error) {
	job, exists := m.jobs[key]
	return job, exists, nil
}

func (m *MockJobStore) Replace([]interface{}, string) error {
	return nil
}

func (m *MockJobStore) Resync() error {
	return nil
}

func TestCheckExistingJobByGroupKey(t *testing.T) {
	// Create mock job store
	jobStore := NewMockJobStore()

	// Create mock kubernetes client
	client := &kubernetes.Client{
		JobStore: jobStore,
	}

	// Test case 1: No existing jobs
	exists := checkExistingJobByGroupKey(client, "test-group-key")
	assert.False(t, exists, "Should return false when no jobs exist")

	// Test case 2: Job exists with same groupKey and is active
	testGroupKey := "test-group-key"
	hashedGroupKey := utils.HashGroupKey(testGroupKey)
	activeJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job-active",
			Namespace: "default",
			Labels: map[string]string{
				"openfero.io/group-key": hashedGroupKey,
			},
		},
		Status: batchv1.JobStatus{
			Active: 1,
		},
	}
	err := jobStore.Add(activeJob)
	if err != nil {
		t.Fatalf("Failed to add job to mock store: %v", err)
	}

	exists = checkExistingJobByGroupKey(client, "test-group-key")
	assert.True(t, exists, "Should return true when active job with same groupKey exists")

	// Test case 3: Job exists with same groupKey but is completed
	testGroupKey2 := "test-group-key-2"
	hashedGroupKey2 := utils.HashGroupKey(testGroupKey2)
	completedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job-completed",
			Namespace: "default",
			Labels: map[string]string{
				"openfero.io/group-key": hashedGroupKey2,
			},
		},
		Status: batchv1.JobStatus{
			Active:    0,
			Succeeded: 1,
		},
	}
	err = jobStore.Add(completedJob)
	if err != nil {
		t.Fatalf("Failed to add completed job to mock store: %v", err)
	}

	exists = checkExistingJobByGroupKey(client, testGroupKey2)
	assert.False(t, exists, "Should return false when job with same groupKey is completed")

	// Test case 4: Job exists with different groupKey
	exists = checkExistingJobByGroupKey(client, "different-group-key")
	assert.False(t, exists, "Should return false when no job with matching groupKey exists")

	// Test case 5: Job exists with same groupKey but failed
	testGroupKey3 := "test-group-key-3"
	hashedGroupKey3 := utils.HashGroupKey(testGroupKey3)
	failedJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job-failed",
			Namespace: "default",
			Labels: map[string]string{
				"openfero.io/group-key": hashedGroupKey3,
			},
		},
		Status: batchv1.JobStatus{
			Active: 0,
			Failed: 1,
		},
	}
	err = jobStore.Add(failedJob)
	if err != nil {
		t.Fatalf("Failed to add failed job to mock store: %v", err)
	}

	exists = checkExistingJobByGroupKey(client, testGroupKey3)
	assert.False(t, exists, "Should return false when job with same groupKey has failed")
}

func TestAddGroupKeyLabel(t *testing.T) {
	// Test case 1: Job with no existing labels
	job1 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-job-1",
		},
	}

	testGroupKey1 := "test-group-key"
	kubernetes.AddGroupKeyLabel(job1, testGroupKey1)

	assert.NotNil(t, job1.Labels, "Labels should be initialized")
	expectedHash1 := utils.HashGroupKey(testGroupKey1)
	assert.Equal(t, expectedHash1, job1.Labels["openfero.io/group-key"], "GroupKey label should be set to hashed value")

	// Test case 2: Job with existing labels
	job2 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-job-2",
			Labels: map[string]string{
				"existing-label": "existing-value",
			},
		},
	}

	testGroupKey2 := "another-group-key"
	kubernetes.AddGroupKeyLabel(job2, testGroupKey2)

	expectedHash2 := utils.HashGroupKey(testGroupKey2)
	assert.Equal(t, expectedHash2, job2.Labels["openfero.io/group-key"], "GroupKey label should be set to hashed value")
	assert.Equal(t, "existing-value", job2.Labels["existing-label"], "Existing labels should be preserved")

	// Test case 3: Empty groupKey should not add label
	job3 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-job-3",
		},
	}

	kubernetes.AddGroupKeyLabel(job3, "")

	if job3.Labels != nil {
		_, exists := job3.Labels["openfero.io/group-key"]
		assert.False(t, exists, "GroupKey label should not be added when groupKey is empty")
	}
}
