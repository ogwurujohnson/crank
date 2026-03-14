# Security considerations

## Configuration

- **Config file path**: `QuickStart(configPath)` and internal `config.Load(path)` reject paths that are or start with `..` (traversal). Do not pass untrusted user input as the config path.
- **YAML content**: Config files are trusted input. Do not load YAML from untrusted sources.
- **Environment variables**: `REDIS_URL` and `NATS_URL` can contain credentials. Prefer environment-based config over config files when secrets are involved.

## Broker and queues

- **Broker URL**: Redis (and future broker) URLs may contain passwords. Keep config and env vars restricted; avoid logging full URLs.
- **Queue names**: Queue names are used in Redis keys (e.g. `queue:<name>`). Avoid user-controlled queue names that could be used for key-space abuse (e.g. very long names or special characters). Prefer fixed, allowlisted names.
- **DeleteKey**: The broker’s `DeleteKey` is used internally (e.g. queue clear). Do not expose it with arbitrary user-controlled keys in production.

## TLS

- **TLS**: Use `rediss://` or enable TLS in config for Redis in production. Avoid `TLSInsecureSkipVerify` outside development.
- **Min version**: The Redis client uses TLS 1.2 minimum when TLS is enabled.

## Logging and redaction

- **Job arguments**: Failed jobs are logged with arguments passed through the configured **redactor**. Default is masking; use `SetRedactor` to customize. Avoid `NoopRedactor` if logs may contain sensitive data.
- **Panics**: The recovery middleware logs panic value and stack trace. Ensure logs are protected and not exposed to untrusted parties.

## Validation

- Use **validators** (e.g. `ClassAllowlist`, `MaxArgsCount`, `MaxPayloadSize`) to restrict which jobs can run and how large payloads can be. Set a global validator with `SetValidator` to enforce policy before job execution.

## Testing

- **NewTestEngine** and the in-memory broker do not use Redis. They are intended for tests only; do not use them in production code paths.

## Reporting vulnerabilities

If you discover a security issue, please report it privately rather than in a public issue.

**Maintainer:** ogwurujohnson@gmail.com
