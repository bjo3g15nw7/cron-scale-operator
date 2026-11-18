// Package v1alpha1 contains the CronScale API.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group and version served by this operator.
	GroupVersion = schema.GroupVersion{Group: "platform.milan.dev", Version: "v1alpha1"}

	// SchemeBuilder registers the CronScale types with a runtime scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the CronScale types to a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Window is one recurring interval during which a target keeps a fixed
// replica count. Start and End are standard five-field cron expressions in
// the schedule timezone.
type Window struct {
	// Name identifies the window in status and conditions.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Start is the cron expression that opens the window.
	// +kubebuilder:validation:Required
	Start string `json:"start"`

	// End is the cron expression that closes the window.
	// +kubebuilder:validation:Required
	End string `json:"end"`

	// Replicas is the replica count applied while the window is open.
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`
}

// CronScaleSpec declares the desired scheduled scaling behaviour.
type CronScaleSpec struct {
	// Target is the workload that receives the replica count.
	// +kubebuilder:validation:Required
	Target TargetRef `json:"target"`

	// Windows are evaluated in order; the window that opened most recently
	// wins. An empty list leaves the target at MinReplicas.
	// +kubebuilder:validation:MinItems=0
	Windows []Window `json:"windows"`

	// Timezone is an IANA zone name used to evaluate the cron expressions.
	// +kubebuilder:default="UTC"
	Timezone string `json:"timezone,omitempty"`

	// MinReplicas bounds every window's replica count.
	// +kubebuilder:default=0
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// MaxReplicas bounds every window's replica count.
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// Suspend freezes reconciliation and leaves the target untouched.
	Suspend bool `json:"suspend,omitempty"`
}

// TargetRef points at the scaled workload.
type TargetRef struct {
	// Kind is Deployment or StatefulSet; defaults to Deployment.
	// +kubebuilder:validation:Enum=Deployment;StatefulSet
	// +kubebuilder:default="Deployment"
	Kind string `json:"kind,omitempty"`

	// Name is the workload name in the CronScale namespace.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// CronScaleStatus reports the evaluated window and replica decision.
type CronScaleStatus struct {
	// ActiveWindow is the window currently in effect, empty when none is open.
	ActiveWindow string `json:"activeWindow,omitempty"`

	// DesiredReplicas is the clamped replica count the operator applied.
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// ObservedGeneration is the spec generation last reconciled.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions carry Ready, Progressing and Degraded signals.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// CronScale is a scheduled replica policy for one workload.
type CronScale struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronScaleSpec   `json:"spec,omitempty"`
	Status CronScaleStatus `json:"status,omitempty"`
}

// CronScaleList is a list of CronScale objects.
type CronScaleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronScale `json:"items"`
}

// DeepCopyInto copies the receiver into out.
func (in *CronScale) DeepCopyInto(out *CronScale) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy returns a deep copy of the CronScale.
func (in *CronScale) DeepCopy() *CronScale {
	if in == nil {
		return nil
	}
	out := new(CronScale)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject satisfies runtime.Object.
func (in *CronScale) DeepCopyObject() runtime.Object { return in.DeepCopy() }

// DeepCopyInto copies the receiver into out.
func (in *CronScaleList) DeepCopyInto(out *CronScaleList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]CronScale, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy returns a deep copy of the list.
func (in *CronScaleList) DeepCopy() *CronScaleList {
	if in == nil {
		return nil
	}
	out := new(CronScaleList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject satisfies runtime.Object.
func (in *CronScaleList) DeepCopyObject() runtime.Object { return in.DeepCopy() }

// DeepCopyInto copies the receiver into out.
func (in *CronScaleSpec) DeepCopyInto(out *CronScaleSpec) {
	*out = *in
	out.Target = in.Target
	if in.Windows != nil {
		out.Windows = make([]Window, len(in.Windows))
		copy(out.Windows, in.Windows)
	}
}

// DeepCopyInto copies the receiver into out.
func (in *CronScaleStatus) DeepCopyInto(out *CronScaleStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}

func init() {
	SchemeBuilder.Register(&CronScale{}, &CronScaleList{})
}
