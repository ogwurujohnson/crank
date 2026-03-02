package queue

import (
	"fmt"

	"github.com/ogwurujohnson/crank/internal/broker"
)

type Queue struct {
	name   string
	broker broker.Broker
}

func NewQueue(name string, b broker.Broker) *Queue {
	return &Queue{
		name:   name,
		broker: b,
	}
}

func (q *Queue) Size() (int64, error) {
	return q.broker.GetQueueSize(q.name)
}

func (q *Queue) Name() string {
	return q.name
}

func (q *Queue) Clear() error {
	return q.broker.DeleteKey("queue:" + q.name)
}

type Stats struct {
	Processed int64            `json:"processed"`
	Retry     int64            `json:"retry"`
	Dead      int64            `json:"dead"`
	Queues    map[string]int64 `json:"queues"`
}

func getInt64(m map[string]interface{}, key string) (int64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("stats: missing or nil %q", key)
	}
	switch n := v.(type) {
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	case float64:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("stats: %q has invalid type %T", key, v)
	}
}

func getQueuesMap(m map[string]interface{}, key string) (map[string]int64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, fmt.Errorf("stats: missing or nil %q", key)
	}
	if q, ok := v.(map[string]int64); ok {
		out := make(map[string]int64, len(q))
		for k, n := range q {
			out[k] = n
		}
		return out, nil
	}
	raw, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("stats: %q is not a map", key)
	}
	out := make(map[string]int64, len(raw))
	for k, val := range raw {
		if val == nil {
			out[k] = 0
			continue
		}
		switch n := val.(type) {
		case int64:
			out[k] = n
		case int:
			out[k] = int64(n)
		case float64:
			out[k] = int64(n)
		default:
			return nil, fmt.Errorf("stats: %q[%s] has invalid type %T", key, k, val)
		}
	}
	return out, nil
}

func GetStats(b broker.Broker) (*Stats, error) {
	statsMap, err := b.GetStats()
	if err != nil {
		return nil, err
	}

	processed, err := getInt64(statsMap, "processed")
	if err != nil {
		return nil, err
	}
	retry, err := getInt64(statsMap, "retry")
	if err != nil {
		return nil, err
	}
	dead, err := getInt64(statsMap, "dead")
	if err != nil {
		return nil, err
	}
	queues, err := getQueuesMap(statsMap, "queues")
	if err != nil {
		return nil, err
	}

	return &Stats{
		Processed: processed,
		Retry:     retry,
		Dead:      dead,
		Queues:    queues,
	}, nil
}
