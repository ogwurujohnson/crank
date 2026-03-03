package payload

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNewJob(t *testing.T) {
	c := qt.New(t)
	j := NewJob("EmailWorker", "default", 123, "foo")
	c.Assert(j.Class, qt.Equals, "EmailWorker")
	c.Assert(j.Queue, qt.Equals, "default")
	c.Assert(j.Retry, qt.Equals, 5)
	c.Assert(j.RetryCount, qt.Equals, 0)
	c.Assert(j.JID, qt.Not(qt.Equals), "")
	c.Assert(len(j.Args), qt.Equals, 2)
	c.Assert(j.Metadata, qt.Not(qt.IsNil))
}

func TestJob_SetRetry_SetBacktrace(t *testing.T) {
	c := qt.New(t)
	j := NewJob("W", "q")
	j.SetRetry(3).SetBacktrace(true)
	c.Assert(j.Retry, qt.Equals, 3)
	c.Assert(j.Backtrace, qt.IsTrue)
}

func TestJob_ToJSON_FromJSON_RoundTrip(t *testing.T) {
	c := qt.New(t)
	j := NewJob("Worker", "default", 1, "two")
	data, err := j.ToJSON()
	c.Assert(err, qt.IsNil)
	j2, err := FromJSON(data)
	c.Assert(err, qt.IsNil)
	c.Assert(j2.Class, qt.Equals, j.Class)
	c.Assert(j2.Queue, qt.Equals, j.Queue)
	c.Assert(j2.JID, qt.Equals, j.JID)
	c.Assert(len(j2.Args), qt.Equals, 2)
}

func TestFromJSON_Invalid(t *testing.T) {
	c := qt.New(t)
	_, err := FromJSON([]byte("not json"))
	c.Assert(err, qt.IsNotNil)
}

func TestFromJSON_ValidMinimal(t *testing.T) {
	c := qt.New(t)
	data := []byte(`{"jid":"id1","class":"W","args":[1,2],"queue":"q","retry":5,"retry_count":0,"created_at":0,"enqueued_at":0,"backtrace":false}`)
	j, err := FromJSON(data)
	c.Assert(err, qt.IsNil)
	c.Assert(j.JID, qt.Equals, "id1")
	c.Assert(j.Class, qt.Equals, "W")
	c.Assert(j.Queue, qt.Equals, "q")
	c.Assert(len(j.Args), qt.Equals, 2)
}

func TestJob_String(t *testing.T) {
	c := qt.New(t)
	j := NewJob("W", "default")
	s := j.String()
	c.Assert(s, qt.Not(qt.Equals), "")
	c.Assert(len(s) >= 10, qt.IsTrue)
}
