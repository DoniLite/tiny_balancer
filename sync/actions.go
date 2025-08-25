// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sync

import (
	"encoding/json"
	"fmt"
)

type Action_Type int

const (
	CREATE_SERVER Action_Type = iota
	ROLLBACK_SERVER
	ADD_SERVER
	KILL_SERVER
	REBOOT_SERVER
	ERROR
)

type Action struct {
	/*
		Based on the Action_Type Enum can be
		    CREATE_SERVER
		    ROLLBACK_SERVER
			ADD_SERVER
			KILL_SERVER
			REBOOT_SERVER
			ERROR
	*/
	Type    Action_Type     `json:"type" yaml:"type"`
	Payload json.RawMessage `json:"payload" yaml:"payload"` // The associated payload to provide with this action
}

func (action *Action) Fill(actionType Action_Type, payload []byte) {
	action.Type = actionType
	action.Payload = payload
}

func (action *Action) Deserialize(target any) error {
	if len(action.Payload) == 0 {
		return fmt.Errorf("the length of the payload is empty for the action type: %d", action.Type)
	}
	if err := json.Unmarshal(action.Payload, target); err != nil {
		return err
	}
	return nil
}

func (action *Action) AddPayload(payload any) error {
	if jsonBytes, err := json.Marshal(payload); err == nil {
		action.Payload = jsonBytes
		return nil
	} else {
		return err
	}
}

func (m *Message) DecodePayload(target any) error {
	if len(m.Action.Payload) == 0 {
		return fmt.Errorf("message payload is empty for type %d", m.Action.Type)
	}
	if err := m.Action.Deserialize(target); err != nil {
		return fmt.Errorf("failed to unmarshal payload for type %d: %w", m.Action.Type, err)
	}
	return nil
}
