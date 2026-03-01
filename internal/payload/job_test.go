package payload

import (
	"testing"
)

func TestNewJob(t *testing.T) {
	j := NewJob("EmailWorker", "default", 123, "foo")
	if j.Class != "EmailWorker" {
		t.Errorf("Class: got %q", j.Class)
	}
	if j.Queue != "default" {
		t.Errorf("Queue: got %q", j.Queue)
	}
	if j.Retry != 5 {
		t.Errorf("Retry: got %d", j.Retry)
	}
	if j.RetryCount != 0 {
		t.Errorf("RetryCount: got %d", j.RetryCount)
	}
	if j.JID == "" {
		t.Error("JID should be set")
	}
	if len(j.Args) != 2 {
		t.Errorf("Args len: got %d", len(j.Args))
	}
	if j.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestJob_SetRetry_SetBacktrace(t *testing.T) {
	j := NewJob("W", "q")
	j.SetRetry(3).SetBacktrace(true)
	if j.Retry != 3 {
		t.Errorf("Retry: got %d", j.Retry)
	}
	if !j.Backtrace {
		t.Error("Backtrace should be true")
	}
}

func TestJob_ToJSON_FromJSON_RoundTrip(t *testing.T) {
	j := NewJob("Worker", "default", 1, "two")
	data, err := j.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	j2, err := FromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if j2.Class != j.Class || j2.Queue != j.Queue || j2.JID != j.JID {
		t.Errorf("round trip mismatch: %+v vs %+v", j, j2)
	}
	if len(j2.Args) != 2 {
		t.Errorf("Args len: got %d", len(j2.Args))
	}
}

func TestFromJSON_Invalid(t *testing.T) {
	_, err := FromJSON([]byte("not json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFromJSON_ValidMinimal(t *testing.T) {
	// JSON numbers unmarshal to float64 in Go
	data := []byte(`{"jid":"id1","class":"W","args":[1,2],"queue":"q","retry":5,"retry_count":0,"created_at":0,"enqueued_at":0,"backtrace":false}`)
	j, err := FromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if j.JID != "id1" || j.Class != "W" || j.Queue != "q" {
		t.Errorf("got %+v", j)
	}
	// Args from JSON are []interface{} with float64 for numbers
	if len(j.Args) != 2 {
		t.Errorf("Args len: got %d", len(j.Args))
	}
}

func TestJob_String(t *testing.T) {
	j := NewJob("W", "default")
	s := j.String()
	if s == "" || len(s) < 10 {
		t.Errorf("String(): got %q", s)
	}
}
