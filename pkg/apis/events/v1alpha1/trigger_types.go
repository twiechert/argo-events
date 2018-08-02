package v1alpha1

import (
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Trigger represents an intention that a particular event should be consumed.
type Trigger struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              TriggerSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            TriggerStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// TriggerSpec specifies a reference to an event and how to execute the trigger.
type TriggerSpec struct {
	// Event is a reference to the event which holds the set of conditions for this trigger.
	Event *api.LocalObjectReference `json:"event,omitempty"`

	// Executor is the process by which this trigger consumes the Event
	// and performs an action.
	Executor *api.Container `json:"executor,omitempty"`
}

// TriggerStatus is the status of a Trigger.
type TriggerStatus struct {
	// States represents the latest available observations of a trigger's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	States []TriggerState `json:"states,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type TriggerStatePhase string

const (
	// TriggerComplete
	TriggerComplete TriggerStatePhase = "Complete"

	TriggerPending TriggerStatePhase = "Pending"

	TriggerFailed TriggerStatePhase = "Failed"
)

type TriggerState struct {
	Phase TriggerStatePhase `json:"phase"`

	Status api.ConditionStatus `json:"status"`

	Reason string `json:"reason,omitempty"`

	Message string `json:"message,omitempty"`
}
