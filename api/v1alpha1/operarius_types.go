/*
Copyright 2025 OpenFero.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  This is scaffolding for you to own.
// NOTE: json tags are required.  Any new fields you add must have json:"-" or json:"fieldName" tags.

// AlertSelector defines which alerts trigger this Operarius
type AlertSelector struct {
	// AlertName to match (required)
	AlertName string `json:"alertname"`

	// Status defines which alert status to match (firing/resolved)
	// +kubebuilder:validation:Enum=firing;resolved
	Status string `json:"status"`

	// Labels defines additional label selectors for the alert
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// DeduplicationConfig defines deduplication settings
type DeduplicationConfig struct {
	// Enabled indicates whether deduplication is enabled
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// TTL defines the time to live for deduplication in seconds
	// +optional
	TTL int32 `json:"ttl,omitempty"`
}

// OperariusSpec defines the desired state of Operarius
type OperariusSpec struct {
	// AlertSelector defines which alerts trigger this Operarius
	AlertSelector AlertSelector `json:"alertSelector"`

	// JobTemplate describes the job that will be created when executing a remediation.
	// This embeds the full Kubernetes JobTemplateSpec to ensure 100% compatibility
	// with native Kubernetes Jobs.
	JobTemplate batchv1.JobTemplateSpec `json:"jobTemplate"`

	// Priority defines the priority of this Operarius (higher number = higher priority)
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// Enabled indicates whether this Operarius is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Deduplication defines deduplication settings for this Operarius
	// +optional
	Deduplication *DeduplicationConfig `json:"deduplication,omitempty"`
}

// OperariusConditionType represents the type of condition
type OperariusConditionType string

const (
	// OperariusConditionReady indicates the Operarius is ready to execute
	OperariusConditionReady OperariusConditionType = "Ready"
	// OperariusConditionExecuting indicates the Operarius is currently executing
	OperariusConditionExecuting OperariusConditionType = "Executing"
	// OperariusConditionFailed indicates the last execution failed
	OperariusConditionFailed OperariusConditionType = "Failed"
)

// OperariusCondition represents a condition of an Operarius
type OperariusCondition struct {
	// Type of Operarius condition
	Type OperariusConditionType `json:"type"`

	// Status of the condition, one of True, False, Unknown
	Status metav1.ConditionStatus `json:"status"`

	// Last time the condition transitioned from one status to another
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Unique, one-word, CamelCase reason for the condition's last transition
	// +optional
	Reason string `json:"reason,omitempty"`

	// Human-readable message indicating details about last transition
	// +optional
	Message string `json:"message,omitempty"`
}

// OperariusStatus defines the observed state of Operarius
type OperariusStatus struct {
	// Conditions represent the latest available observations of an Operarius's state
	// +optional
	Conditions []OperariusCondition `json:"conditions,omitempty"`

	// LastExecutionTime represents the last time a job was created from this Operarius
	// +optional
	LastExecutionTime *metav1.Time `json:"lastExecutionTime,omitempty"`

	// ExecutionCount represents the total number of jobs created from this Operarius
	// +optional
	ExecutionCount int32 `json:"executionCount,omitempty"`

	// LastExecutedJobName represents the name of the last job created
	// +optional
	LastExecutedJobName string `json:"lastExecutedJobName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=op
// +kubebuilder:printcolumn:name="Alert",type=string,JSONPath=`.spec.alertSelector.alertname`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.spec.alertSelector.status`
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Executions",type=integer,JSONPath=`.status.executionCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Operarius is the Schema for the operarii API
type Operarius struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperariusSpec   `json:"spec,omitempty"`
	Status OperariusStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OperariusList contains a list of Operarius
type OperariusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Operarius `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Operarius{}, &OperariusList{})
}
