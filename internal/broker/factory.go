package broker

import (
	"fmt"
	"strings"
)

// Open creates a Broker from a backend kind, URL, and connection options.
// Kind is set by config (broker: redis|nats|rabbitmq) so users choose the backend explicitly;
// the URL and opts come from the matching config section (redis, nats, etc.).
//
//   - kind "redis" → Redis broker; opts must include Timeout, UseTLS, TLSInsecureSkipVerify (from redis config).
//   - kind "nats"  → NATS broker (not yet implemented).
//   - kind "rabbitmq" → RabbitMQ broker (not yet implemented).
//
// If kind is empty, Open falls back to inferring from URL scheme for backward compatibility.
func Open(kind string, rawURL string, opts ConnOptions) (Broker, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("broker: URL is empty")
	}

	kind = strings.TrimSpace(strings.ToLower(kind))
	if kind == "" {
		kind = inferKindFromURL(rawURL)
	}

	switch kind {
	case "redis":
		return openRedis(rawURL, opts)
	case "nats":
		return openNats(rawURL, opts)
	case "rabbitmq":
		return nil, fmt.Errorf("broker: RabbitMQ backend not yet implemented")
	default:
		return nil, fmt.Errorf("broker: unsupported broker %q (use redis, nats, or rabbitmq)", kind)
	}
}

// inferKindFromURL returns a broker kind from URL scheme for backward compatibility
// when kind is not set (e.g. programmatic New with only URL).
func inferKindFromURL(rawURL string) string {
	switch {
	case strings.HasPrefix(strings.ToLower(rawURL), "redis://"), strings.HasPrefix(strings.ToLower(rawURL), "rediss://"):
		return "redis"
	case strings.HasPrefix(strings.ToLower(rawURL), "nats://"):
		return "nats"
	case strings.HasPrefix(strings.ToLower(rawURL), "amqp://"), strings.HasPrefix(strings.ToLower(rawURL), "amqps://"):
		return "rabbitmq"
	default:
		return ""
	}
}

func openNats(rawURL string, opts ConnOptions) (Broker, error) {
	return nil, fmt.Errorf("broker: NATS backend not yet implemented")
}

func openRedis(rawURL string, opts ConnOptions) (Broker, error) {
	return NewRedisBrokerWithConfig(RedisBrokerConfig{
		URL:                   rawURL,
		Timeout:               opts.Timeout,
		UseTLS:                opts.UseTLS,
		TLSInsecureSkipVerify: opts.TLSInsecureSkipVerify,
	})
}
