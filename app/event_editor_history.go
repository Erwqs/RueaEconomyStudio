package app

import (
	eventeditor "etools/event_editor"
	"sync"
)

var edits = make([]*eventeditor.Event, 0)
var undoOverwriited = false
var mu sync.Mutex

func AppendEdit(event *eventeditor.Event) {
	if event == nil {
		return
	}

	// Append the event to the edits slice
	mu.Lock()
	edits = append(edits, event)
	mu.Unlock()
}

func GetEdits() []*eventeditor.Event {
	mu.Lock()
	defer mu.Unlock()
	return edits
}

func Undo(n uint) *eventeditor.Event {
	if n == 0 || len(edits) == 0 {
		return nil // No edits to undo
	}

	// Out of bounds check
	if n > uint(len(edits)) {
		n = uint(len(edits))
	}

	mu.Lock()
	defer mu.Unlock()
	// Return the nth edits
	pointInTimeEdit := edits[len(edits)-int(n)]

	undoOverwriited = false

	return pointInTimeEdit
}
