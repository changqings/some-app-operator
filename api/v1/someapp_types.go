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

// SomeappSpec defines the desired state of Someapp
type SomeappSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// application name
	AppName string `json:"app_name"`

	// if script, will not create svc and hpa,
	// if api, create svc and hpa
	// +kubebuilder:validation:Enum=script;api
	// +kubebuilder:default=api
	AppType string `json:"app_type"`

	// +kubebuilder:validation:Enum=stable;canary
	// +kubebuilder:default=stable
	// +optional
	AppVersion string `json:"app_version"`

	// stable version, tag like v1.0.0,
	// canary version, tag like canary-v1.0.0

	// +kubebuilder:validation:Pattern=(latest|(canary-)?(v\d+\.\d+\.\d+)(\.\d+)?)
	// +kubebuilder:default=latest
	ImageTag string `json:"image_tag"`

	ImagePullSecret string `json:"image_secret"`

	Containers []core_v1.Container `json:"containers"`

	// useage:  some_volume: configmap-a or some_volume: secret-b
	// +optional
	SomeVolume string `json:"some_volume,omitempty"`

	// usage: hpa_nums: 1-2
	// +kubebuilder:validation:Pattern=^(\d-\d)$
	// +kubebuilder:default=1-2
	// +optional
	HpaNums string `json:"hpa_nums,omitempty"`

	// +kubebuilder:default=100
	// +optional
	HpaCpuPercent int32 `json:"hpa_cpu_percent,omitempty"`

	// if api_type=api and enable_istio, than create vs,dr with stable version
	// +kubebuilder:default=false
	// +optional
	EnableIstio bool `json:"enable_istio,omitempty"`
}

// SomeappStatus defines the observed state of Someapp
type SomeappStatus struct {
	// Important: Run "make" to regenerate code after modifying this file
	Status someAppSts `json:"status"`
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
