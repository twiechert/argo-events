package v1alpha1

import (
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Event describes a set of dependent conditions that it evaluates against the log of events on the wire.
type Event struct {
	v1.TypeMeta   `json:",inline"`
	v1.ObjectMeta `json:"metadata"`
	Spec          EventSpec   `json:"spec"`
	Status        EventStatus `json:"status"`
}

// EventSpec specifies a set of conditions to evaluate for this event.
type EventSpec struct {
	// Deadline is the time after which this event becomes expired if it has not already
	// moved to a Complete phase.
	Deadline *v1.Time `json:"deadline",omitempty`

	// Conditions contains the set of conditions to evaluate for this event.
	Conditions []EventCondition `json:"conditions,omitempty"`
}

// EventCondition defines a condition for this event.
type EventCondition struct {
	// A reference to the sensor producing this event.
	Sensor *api.LocalObjectReference `json:"sensor,omitempty"`

	// Start is the time after which events from the sensor are considered.
	Start *v1.Time `json:"start,omitempty"`

	// Stop is the time before which events from the sensor are considered.
	Stop *v1.Time `json:"stop,omitempty"`
}

// EventStatus records the state of an Event.
type EventStatus struct {
	// States represents the latest available observations of an event's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	States []EventState `json:"states,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// EventStatePhase describes the possible phases for an Event.
type EventStatePhase string

const (
	// EventPhaseComplete specifies that the event conditions have been satisfied.
	EventPhaseComplete EventStatePhase = "Complete"

	// EventPhaseExpired specifies that the event has expired.
	EventPhaseExpired EventStatePhase = "Expired"

	// EventPhasePending specifies that the event conditions have not been satisfied.
	EventPhasePending EventStatePhase = "Pending"
)

// EventState defines the state for an Event.
type EventState struct {
	// Phase of the EventState
	Phase EventStatePhase `json:"phase"`

	// Status of the EventState
	Status api.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

	// +optional
	Reason string `json:"reason,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`
}
