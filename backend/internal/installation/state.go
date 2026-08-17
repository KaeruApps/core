package installation

import (
	"context"
	"fmt"
)

type State string

const (
	StateRequired    State = "required"
	StateConfiguring State = "configuring"
	StateRestoring   State = "restoring"
	StateReady       State = "ready"
)

type StateReader interface {
	State(context.Context) (State, error)
}

func (state State) Valid() bool {
	switch state {
	case StateRequired, StateConfiguring, StateRestoring, StateReady:
		return true
	default:
		return false
	}
}

func ParseState(value string) (State, error) {
	state := State(value)
	if !state.Valid() {
		return "", fmt.Errorf("invalid installation state %q", value)
	}
	return state, nil
}
