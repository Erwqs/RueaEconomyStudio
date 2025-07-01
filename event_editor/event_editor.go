package eventeditor

import (
	"etools/typedef"
)

type TriggerType int

const (
	TriggerTypeNone TriggerType = iota
	TriggerTypeTime
	TriggerTypeResource
	TriggerTypeTerritoryUpdate
	TriggerTypeRouteUpdate
	TriggerTypeJS         // Triggered by user script
	TriggerTypeEvent      // Triggered by another event
)

// TODO: event editor
type EventEditor struct{}

type Event struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	Affecting []*typedef.Territory `json:"affecting"`
	Trigger   TriggerType          `json:"trigger"`

	DT uint64 `json:"dt"` // Delta time
}

type EventData struct {
	typedef.TerritoryOptions
	typedef.Guild
}
