/*
Copyright 2022.

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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// UnitSpec defines the desired state of Unit
type UnitSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// category two types: Deployment / StatefulSet, 在admissionWebhook里面做校验
	Category string `json:"category"`

	//这两个字段在mutateWebhook中有默认填充
	Replicas *int32                `json:"replicas,omitempty"`
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Template describes the pods that will be created
	Template         v1.PodTemplateSpec       `json:"template"`
	RelationResource UnitRelationResourceSpec `json:"relationResource,omitempty"`
}

type UnitRelationResourceSpec struct {
	Service *OwnService `json:"serviceInfo,omitempty"`
	PVC     *OwnPVC     `json:"pvcInfo,omitempty"`
	Ingress *OwnIngress `json:"ingressInfo,omitempty"`
}

// UnitStatus defines the observed state of Unit
type UnitStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Replicas       *int32      `json:"replicas,omitempty"`
	Selector       string      `json:"selector"`
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	BaseDeployment         appsv1.DeploymentStatus    `json:"deployment,omitempty"`
	BaseStatefulSet        appsv1.StatefulSetStatus   `json:"statefulSet,omitempty"`
	RelationResourceStatus UnitRelationResourceStatus `json:"relationResourceStatus,omitempty"`
}

type UnitRelationResourceStatus struct {
	Service  UnitRelationServiceStatus      `json:"service,omitempty"`
	Ingress  []v1beta1.IngressRule          `json:"ingress,omitempty"`
	Endpoint []UnitRelationEndpointStatus   `json:"endpoint,omitempty"`
	PVC      v1.PersistentVolumeClaimStatus `json:"pvc,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Unit is the Schema for the units API
type Unit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnitSpec   `json:"spec,omitempty"`
	Status UnitStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UnitList contains a list of Unit
type UnitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Unit `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Unit{}, &UnitList{})
}
