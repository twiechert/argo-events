package v1alpha1

import (
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Sensor describes the source of an event.
type Sensor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              SensorSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            SensorStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// SensorList is the list of Sensor resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SensorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Sensor `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// SensorSpec specifies how the listener for the sensor is run and which
// volumes to be mounted to it's container.
type SensorSpec struct {
	// todo: include some reference to the types of events that this sensor produces

	// Listener is the container for this sensor that is responsible
	// for listening for events.
	Listener *api.Container `json:"listener,omitempty"`

	// Volumes to be mounted inside the listener container
	Volumes *[]api.Volume `json:"volumes,omitempty"`
}

// SensorStatus represents the health and status of the sensor
type SensorStatus struct {
	// Represents the latest available observations of a sensor's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	States []SensorState `json:"states,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// SensorStatePhase describes the possible phases for a sensor.
type SensorStatePhase string

const (
	// SensorRunning means the Sensor Listener pod is up and running.
	SensorRunning SensorStatePhase = "Running"

	// SensorPending means the Sensor Listener pod is pending.
	SensorPending SensorStatePhase = "Pending"

	// SensorFailed means the Sensor Listener has failed.
	SensorFailed SensorStatePhase = "Failed"
)

// SensorState describes the state of the sensor at a point in time.
type SensorState struct {
	Phase SensorStatePhase `json:"phase"`

	Status api.ConditionStatus `json:"status"`

	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	Reason string `json:"reason,omitempty"`

	Message string `json:"message,omitempty"`
}
