package eventeditor

import (
	"RueaES/eruntime"
	"RueaES/typedef"
)

func Run() {
	go func() {
		tick := <-eruntime.GetStateTick()
		if event, exists := storedEvents[tick]; exists {
			ExecuteEvent(event)
		}
	}()
}

func ExecuteEvent(event *Event) {
	if event == nil {
		return
	}

	if len(event.Affecting) == 0 {
		return // No territories to affect
	}

	if event.Trigger != TriggerStateTick {
		return // Only handle state tick events for now
	}

	if event.Data.Options != nil {
		for _, territory := range event.Affecting {
			// Apply options to the territory
			eruntime.SetT(territory, *event.Data.Options)
		}
	}

	if event.Data.Guild != nil {
		batch := make(map[string]*typedef.Guild)
		for _, territory := range event.Affecting {
			batch[territory.ID] = event.Data.Guild
		}

		eruntime.SetGuildBatch(batch)
	}

	if event.Data.Storage != nil {
		for _, affected := range event.Affecting {
			eruntime.ModifyStorageStateT(affected, *event.Data.Storage)
		}
	}
}
