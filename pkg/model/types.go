package model

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TODO: Can use generators instead

var (
	SchemeGroupVersion = schema.GroupVersion{Group: "k8schedul8r.io", Version: "v1alpha1"}

	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func GroupResource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

type ScheduledResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ScheduledResourceSpec `json:"spec"`
}

func (in *ScheduledResource) DeepCopyInto(out *ScheduledResource) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

func (in *ScheduledResource) DeepCopy() *ScheduledResource {
	if in == nil {
		return nil
	}
	out := new(ScheduledResource)
	in.DeepCopyInto(out)
	return out
}

func (in *ScheduledResource) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

type ScheduledResourceSpec struct {
	Target           ResourceTarget `json:"target"`
	OriginalReplicas int32          `json:"originalReplicas"`
	Windows          []Window       `json:"windows"`
}

func (in *ScheduledResourceSpec) DeepCopyInto(out *ScheduledResourceSpec) {
	*out = *in
	out.Target = in.Target
	if in.Windows != nil {
		in, out := &in.Windows, &out.Windows
		*out = make([]Window, len(*in))
		copy(*out, *in)
	}
}

type ResourceTarget struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
}

type Window struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`
	Replicas  int32 `json:"replicas"`
}

type ScheduledResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScheduledResource `json:"items"`
}

func (in *ScheduledResourceList) DeepCopyInto(out *ScheduledResourceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ScheduledResource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *ScheduledResourceList) DeepCopy() *ScheduledResourceList {
	if in == nil {
		return nil
	}
	out := new(ScheduledResourceList)
	in.DeepCopyInto(out)
	return out
}

func (in *ScheduledResourceList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ScheduledResource{},
		&ScheduledResourceList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
