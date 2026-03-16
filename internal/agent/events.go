package agent

import "github.com/yogirk/cascade/pkg/types"

// EventHandler receives agent events. TUI and one-shot mode implement this.
type EventHandler interface {
	HandleEvent(event types.Event)
}

// EventChan is a channel-based EventHandler.
type EventChan chan types.Event

// HandleEvent sends the event to the channel.
func (ch EventChan) HandleEvent(event types.Event) {
	ch <- event
}
