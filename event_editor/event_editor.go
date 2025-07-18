package eventeditor

import (
	"etools/typedef"
)

// State tick to event mapping
var storedEvents = make(map[uint64]*Event)

type TriggerType int

const (
	TriggerStateTick TriggerType = iota
	TriggerUser
	TriggerScript
)

// TODO: event editor
type EventEditor struct{}

type Event struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	Affecting     []*typedef.Territory `json:"-"`
	AffectingJSON []string             `json:"affecting"` // Territory IDs in JSON format
	Trigger       TriggerType          `json:"trigger"`

	Data *EventData `json:"data"` // Data to be applied to the affected territories

	DT uint64 `json:"dt"` // Delta time
}

type EventData struct {
	Options *typedef.TerritoryOptions        `json:"options"` // Nil if unchanged
	Guild   *typedef.Guild                   `json:"guild"`   // Nil if unchanged
	Storage *typedef.BasicResourcesInterface `json:"storage"` // Nil if unchanged
}

func (e *Event) QueueEvent() {
	storedEvents[e.DT] = e
}

func NewEvent(id, name, description string, trigger TriggerType, affecting []*typedef.Territory, data *EventData) *Event {
	event := &Event{
		ID:          id,
		Name:        name,
		Description: description,
		Affecting:   affecting,
		Trigger:     trigger,
		Data:        data,
	}

	if len(affecting) > 0 {
		for _, territory := range affecting {
			event.AffectingJSON = append(event.AffectingJSON, territory.ID)
		}
	}

	return event
}
