/*
Copyright 2023 changqings.

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

package v1

import (
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// x-kubernetes-validations is beta in k8s 1.25, when < 1.24, default --feature-gates is false

// SomeappSpec defines the desired state of Someapp
type SomeappSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// application name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="appName is immutable"
	AppName string `json:"appName"`

	//  prefix has api, then create svc, changed not changed
	// +kubebuilder:default=api
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="appType is immutable"
	AppType string `json:"appType"`

	// should be only in (stable,canary), default appVersion=stable,
	// when canaryTAg!=stable, then appVerson=canary,
	// can not changed
	// +kubebuilder:validation:Enum=stable;canary
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="appVersion is immutable"
	// +optional
	AppVersion string `json:"appVersion"`

	// used in labels,
	// defalut canaryTag=stable, or must like canary-v1.0.0,
	// can not changed
	// +kubebuilder:validation:Pattern=(stable|(canary-v\d+\.\d+\.\d+)(\.\d+)?)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="canaryTag is immutable"
	// +optional
	CanaryTag string `json:"canaryTag"`

	// +optional
	ImagePullSecret string `json:"imageSecret"`

	// k8s standard containers resources
	Containers []core_v1.Container `json:"containers"`

	// only use configmap or secret,
	// like configmap-a, secret-b
	// +optional
	SomeVolume string `json:"someVolume,omitempty"`

	// create hpa, with min-->max
	// +kubebuilder:validation:Pattern=\d+\->\d+
	// +optional
	HpaNums string `json:"hpaNums,omitempty"`

	// hpa default cpu usage value, defautl=100
	// +kubebuilder:default=100
	// +optional
	HpaCpuPercent int32 `json:"hpaCpuPercent,omitempty"`

	// only used when apiType has prefix api,
	// appVersion=stable, create stable vs, dr
	// appVersion=canary and CanaryTag have values, create canary vs,dr
	// +kubebuilder:default=false
	// +optional
	EnableIstio bool `json:"enableIstio,omitempty"`
}

// SomeappStatus defines the observed state of Someapp
type SomeappStatus struct {
	// Important: Run "make" to regenerate code after modifying this file
	Status             someAppSts `json:"status"`
	ObservedGeneration int64      `json:"observedGeneration"`
}

type someAppSts struct {
	// Phase running, failed, operating
	Phase string `json:"phase"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Someapp is the Schema for the someapps API
type Someapp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SomeappSpec   `json:"spec,omitempty"`
	Status SomeappStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SomeappList contains a list of Someapp
type SomeappList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Someapp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Someapp{}, &SomeappList{})
}
