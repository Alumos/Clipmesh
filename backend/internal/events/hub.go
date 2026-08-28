package events

import (
	"sync"

	"github.com/Alumos/Clipmesh/backend/internal/model"
)

type Event struct {
	Sequence uint64      `json:"sequence"`
	Type     string      `json:"type"`
	UserID   string      `json:"-"`
	Clip     *model.Clip `json:"clip,omitempty"`
	ID       string      `json:"id,omitempty"`
}

type subscription struct {
	userID string
	client chan Event
}

type Hub struct {
	mu         sync.Mutex
	clients    map[*subscription]struct{}
	history    map[string][]Event
	sequence   uint64
	historyCap int
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*subscription]struct{}),
		history:    make(map[string][]Event),
		historyCap: 128,
	}
}

// Subscribe returns a stream that starts after the supplied event sequence.
// The short in-memory history closes the usual reconnect gap; callers should
// still perform a normal list request after reconnecting because the history
// is intentionally not persisted across process restarts.
func (h *Hub) Subscribe(userID string, after uint64) (chan Event, func()) {
	subscription := &subscription{userID: userID, client: make(chan Event, h.historyCap+16)}
	h.mu.Lock()
	for _, event := range h.history[userID] {
		if event.Sequence > after {
			subscription.client <- event
		}
	}
	h.clients[subscription] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.clients, subscription)
			h.mu.Unlock()
		})
	}
	return subscription.client, unsubscribe
}

func (h *Hub) Publish(event Event) {
	h.mu.Lock()
	h.sequence++
	event.Sequence = h.sequence
	if event.UserID != "" {
		history := append(h.history[event.UserID], event)
		if len(history) > h.historyCap {
			history = history[len(history)-h.historyCap:]
		}
		h.history[event.UserID] = history
	}
	recipients := make([]chan Event, 0, len(h.clients))
	for subscription := range h.clients {
		if event.UserID != "" && subscription.userID == event.UserID {
			recipients = append(recipients, subscription.client)
		}
	}
	h.mu.Unlock()

	for _, client := range recipients {
		select {
		case client <- event:
		default:
			// A slow browser must not block writes for every other device.
		}
	}
}

func (h *Hub) Latest(userID string) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	history := h.history[userID]
	if len(history) == 0 {
		return 0
	}
	return history[len(history)-1].Sequence
}
