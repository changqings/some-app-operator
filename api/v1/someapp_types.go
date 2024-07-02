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

var (
	AppTypeApi    = "api"
	AppTypeScript = "script"
	StableStage   = "stable"
	CanaryStage   = "canary"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// x-kubernetes-validations is beta in k8s 1.25, when < 1.24, default --feature-gates is false

// Someapp defines a set of deployment,service,hpa and istio vs/dr
type SomeappSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// application name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.name is immutable"
	AppName string `json:"name"`

	// AppType value only in (api,job), value immutable
	// api will create service then will create svc
	// job will not create service, only a deployment, and default one pods
	// +kubebuilder:validation:Enum=api;script
	// +kubebuilder:default=api
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.type is immutable"
	// +optional
	AppType string `json:"type"`

	// used in labels,
	// defalut appVersion=stable, or must like canary-v1.0.0, immutable
	// +kubebuilder:validation:Pattern=(stable|(canary-v\d+\.\d+\.\d+)(\.\d+)?)
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.version is immutable"
	// +kubebuilder:default=stable
	// +optional
	AppVersion string `json:"version"`

	// +optional
	ImagePullSecret string `json:"imageSecret"`

	// k8s standard containers resources
	// +kubebuilder:validation:Required
	Containers []core_v1.Container `json:"containers"`

	// only use configmap or secret,
	// like configmap name a, secret name b
	// +optional
	SomeVolume string `json:"someVolume,omitempty"`

	// create hpa, with min-->max
	// if not set, will not create hpa
	// +kubebuilder:validation:Pattern=\d+\->\d+
	// +optional
	SetHpa string `json:"setHpa,omitempty"`

	// hpa default cpu usage value percent, defautl=100
	// +kubebuilder:default=100
	// +optional
	HpaCpuUsage int32 `json:"hpaCpuUsage,omitempty"`

	// only used when spec.type == api
	// stage=stable, will create stable vs, dr
	// stage=canary, will createOrPatch canary vs,dr
	// canary vs default weight=0
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.enableIstio is immutable"
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
