package events

import "testing"

func TestHubAssignsSequencesAndReplaysOnlyUserEvents(t *testing.T) {
	hub := NewHub()
	hub.Publish(Event{UserID: "user-a", Type: "created"})
	hub.Publish(Event{UserID: "user-b", Type: "created"})
	hub.Publish(Event{UserID: "user-a", Type: "deleted"})

	if got := hub.Latest("user-a"); got != 3 {
		t.Fatalf("Latest(user-a) = %d, want 3", got)
	}

	afterFirst, unsubscribeA := hub.Subscribe("user-a", 1)
	defer unsubscribeA()
	select {
	case event := <-afterFirst:
		if event.Sequence != 3 || event.Type != "deleted" {
			t.Fatalf("replayed event = %#v, want sequence 3 deleted", event)
		}
	default:
		t.Fatal("expected the second user-a event to be replayed")
	}

	userB, unsubscribeB := hub.Subscribe("user-b", 0)
	defer unsubscribeB()
	select {
	case event := <-userB:
		if event.Sequence != 2 || event.UserID != "user-b" {
			t.Fatalf("replayed user-b event = %#v, want sequence 2", event)
		}
	default:
		t.Fatal("expected the user-b event to be replayed")
	}

	select {
	case event := <-afterFirst:
		t.Fatalf("unexpected extra user-a event: %#v", event)
	default:
	}
}
