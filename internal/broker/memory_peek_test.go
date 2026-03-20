package broker

import (
	"testing"

	"github.com/ogwurujohnson/crank/internal/payload"
)

func TestInMemoryBroker_PeekQueue_headOrderAndLimit(t *testing.T) {
	m := NewInMemoryBroker()
	t.Cleanup(func() { _ = m.Close() })

	j1 := payload.NewJob("a", "default", 1)
	j2 := payload.NewJob("b", "default", 2)
	j3 := payload.NewJob("c", "default", 3)
	for _, j := range []*payload.Job{j1, j2, j3} {
		if err := m.Enqueue("default", j); err != nil {
			t.Fatal(err)
		}
	}

	peek, err := m.PeekQueue("default", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(peek) != 2 {
		t.Fatalf("len = %d, want 2", len(peek))
	}
	if peek[0].JID != j1.JID || peek[1].JID != j2.JID {
		t.Fatalf("want [j1,j2] dequeue order, got [%s, %s]", peek[0].JID, peek[1].JID)
	}

	got, q, err := m.Dequeue([]string{"default"}, 0)
	if err != nil || q != "default" || got == nil || got.JID != j1.JID {
		t.Fatalf("Dequeue after peek: got %v %q err=%v", got, q, err)
	}
}
