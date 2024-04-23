// Package Incus contains API return types.
package incus

import (
	"encoding/json"
)

type EventType string

const (
	EventTypeConfig EventType = "config"
	EventTypeDevice EventType = "device"
)

func (et EventType) Valid() bool {
	if et != EventTypeConfig && et != EventTypeDevice {
		return false
	}

	return true
}

type InstanceInfo struct {
	APIVersion   string `json:"api_version"`
	Location     string `json:"location"`
	InstanceType string `json:"instance_type"`
	State        string `json:"state"`
}

type Event struct {
	Timestamp string    `json:"timestamp"`
	Type      EventType `json:"type"`

	Config ConfigUpdateMetadata
	Device DeviceUpdateMetadata
}

type ConfigUpdateMetadata struct {
	Key      string `json:"key"`
	OldValue string `json:"old_value"`
	Value    string `json:"value"`
}

type DeviceUpdateMetadata struct {
	Name   string       `json:"name"`
	Action string       `json:"action"`
	Config DeviceConfig `json:"config"`
}

type DeviceConfig struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

func (e *Event) UnmarshalJSON(data []byte) error {
	var intermediary map[string]json.RawMessage
	if err := json.Unmarshal(data, &intermediary); err != nil {
		return err
	}

	// Unmarshal the guaranteed fields
	if err := json.Unmarshal(intermediary["timestamp"], &e.Timestamp); err != nil {
		return err
	}
	if err := json.Unmarshal(intermediary["type"], &e.Type); err != nil {
		return err
	}

	// Delegate unmarshalling based on event type
	switch e.Type {
	case "config":
		meta, ok := intermediary["metadata"]
		if !ok {
			return nil
		}

		return json.Unmarshal(meta, &e.Config)
	case "device":
		meta, ok := intermediary["metadata"]
		if !ok {
			return nil
		}

		return json.Unmarshal(meta, &e.Device)
	}

	return nil
}
