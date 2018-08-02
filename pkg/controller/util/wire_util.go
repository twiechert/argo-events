package util

import (
	"github.com/argoproj/argo-events/pkg/apis/events/v1alpha1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewWireState creates a new wire state with the provided values and both times set to current time.
func NewWireState(phase v1alpha1.WireStatePhase, status v1.ConditionStatus, reason, message string) *v1alpha1.WireState {
	return &v1alpha1.WireState{
		Phase:              phase,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetWireState returns the wire state with the provided type
func GetWireState(status v1alpha1.WireStatus, phase v1alpha1.WireStatePhase) *v1alpha1.WireState {
	for i := range status.States {
		s := status.States[i]
		if s.Phase == phase {
			return &s
		}
	}
	return nil
}

// SetWireState updates the wire status to include the provided state.
func SetWireState(status *v1alpha1.WireStatus, state v1alpha1.WireState) {
	currentState := GetWireState(*status, state.Phase)
	if currentState != nil && currentState.Status == state.Status && currentState.Reason == state.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentState != nil && currentState.Status == state.Status {
		state.LastTransitionTime = currentState.LastTransitionTime
	}
	newStates := filterOutWireState(status.States, state.Phase)
	status.States = append(newStates, state)
}

// RemoveWireState removes all states from the status with the matching phase
func RemoveWireState(status *v1alpha1.WireStatus, phase v1alpha1.WireStatePhase) {
	status.States = filterOutWireState(status.States, phase)
}

// AggregateWireState computes and sets the overall "Ready" condition of the wire
func AggregateWireState(wire *v1alpha1.Wire) {
	connecting := GetWireState(wire.Status, v1alpha1.WireConnecting)

	var state *v1alpha1.WireState
	if connecting != nil && connecting.Status == v1.ConditionTrue {
		state = NewWireState(v1alpha1.WireReady, v1.ConditionTrue, "", "")
	} else {
		state = NewWireState(v1alpha1.WireReady, v1.ConditionFalse, "", "")
	}
	SetWireState(&wire.Status, *state)
}

// IsWireReady to receive traffic
func IsWireReady(status *v1alpha1.WireStatus) bool {
	p := GetWireState(*status, v1alpha1.WireReady)
	return p != nil && p.Status == v1.ConditionTrue
}

func filterOutWireState(states []v1alpha1.WireState, phase v1alpha1.WireStatePhase) []v1alpha1.WireState {
	var newStates []v1alpha1.WireState
	for _, s := range states {
		if s.Phase == phase {
			continue
		}
		newStates = append(newStates, s)
	}
	return newStates
}
