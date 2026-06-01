package handlers

import (
	"encoding/json"
	"sync"

	"cloudpass/internal/logger"
	"cloudpass/internal/models"
)

type JobEvent struct {
	Type string      `json:"type"`
	Job  *models.Job `json:"job,omitempty"`
}

type EventHub struct {
	mu           sync.RWMutex
	subscribers  map[chan JobEvent]struct{}
	globalEvents chan JobEvent
}

func NewEventHub() *EventHub {
	return &EventHub{
		subscribers:  make(map[chan JobEvent]struct{}),
		globalEvents: make(chan JobEvent, 100),
	}
}

func (h *EventHub) Subscribe() chan JobEvent {
	ch := make(chan JobEvent, 50)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	logger.API.Load().Debug().Msg("client subscribed to job events")
	return ch
}

func (h *EventHub) Unsubscribe(ch chan JobEvent) {
	h.mu.Lock()
	if _, ok := h.subscribers[ch]; ok {
		delete(h.subscribers, ch)
		close(ch)
		logger.API.Load().Debug().Msg("client unsubscribed from job events")
	}
	h.mu.Unlock()
}

func (h *EventHub) Publish(event JobEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	logger.API.Load().Debug().Str("type", event.Type).Str("job_id", event.Job.ID).Msg("publishing job event")

	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
			logger.API.Load().Warn().Msg("failed to send event to subscriber, channel full")
		}
	}
}

func (h *EventHub) BroadcastJobUpdate(job *models.Job) {
	event := JobEvent{
		Type: "job.updated",
		Job:  job,
	}
	h.Publish(event)

	if job.Status == models.JobStatusCompleted {
		event := JobEvent{
			Type: "job.completed",
			Job:  job,
		}
		h.Publish(event)
	} else if job.Status == models.JobStatusFailed {
		event := JobEvent{
			Type: "job.failed",
			Job:  job,
		}
		h.Publish(event)
	}
}

func (h *EventHub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

func MarshalEvent(event JobEvent) ([]byte, error) {
	return json.Marshal(event)
}
