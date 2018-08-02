package v1alpha1

import (
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Wire represents a message bus abstraction layer of communication
// on which events are sent and queued.
type Wire struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              WireSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            WireStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// WireSpec specifies the wire's
type WireSpec struct {
	// Receiver defines the process for how this wire receives events.
	// The Receiver is responsible for
	Receiver api.Container `json:"receiver,omitempty"`

	// Dispatcher defines the process for how events on this wire are forwarded to triggers.
	Dispatcher api.Container `json:"dispatcher,omitempty"`
}

// WireStatus is the status of a Wire
type WireStatus struct {
	// A reference to the k8s Service fronting this wire, if successfully synced.
	Service *api.LocalObjectReference `json:"service,omitempty"`

	// States represents the latest available observations of a wire's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	States []WireState `json:"states,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// WireStatePhase describes the possible phases for a wire.
type WireStatePhase string

const (
	// WireReady is set when the wire is ready to accept events.
	WireReady WireStatePhase = "Ready"

	// WireServiceable means the service fronting the wire exists.
	WireServiceable WireStatePhase = "Serviceable"

	// WireReceiving means the deployment for the wire receiver exists.
	WireReceiving WireStatePhase = "Receiving"

	// WireDispatching means the deployment for the wire dispatcher exists.
	WireDispatching WireStatePhase = "Dispatching"
)

// WireState describes the state of a wire at a point in time.
type WireState struct {
	Phase WireStatePhase `json:"phase"`

	Status api.ConditionStatus `json:"status"`

	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	Reason string `json:"reason,omitempty"`

	Message string `json:"message,omitempty"`
}
