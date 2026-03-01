# Deployment Guide

## Systemd Service

Create `/etc/systemd/system/sidekiq.service`:

```ini
[Unit]
Description=Sidekiq Background Worker
After=syslog.target network.target redis.service

[Service]
Type=simple
User=deploy
WorkingDirectory=/home/deploy/myapp/current
Environment=RAILS_ENV=production
Environment=REDIS_URL=redis://localhost:6379/0
ExecStart=/usr/local/bin/sidekiq -C /home/deploy/myapp/current/config/sidekiq.yml
TimeoutSec=300
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable sidekiq
sudo systemctl start sidekiq
sudo systemctl status sidekiq
```

## Docker

### Dockerfile

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o sidekiq cmd/sidekiq/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/sidekiq .
COPY --from=builder /app/config/sidekiq.yml ./config/

CMD ["./sidekiq", "-C", "config/sidekiq.yml"]
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes

  worker:
    build: .
    environment:
      - REDIS_URL=redis://redis:6379/0
      - RAILS_ENV=production
    depends_on:
      - redis
    deploy:
      replicas: 2
    restart: unless-stopped

volumes:
  redis-data:
```

Build and run:
```bash
docker-compose up -d
```

## Kubernetes

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sidekiq-worker
spec:
  replicas: 3
  selector:
    matchLabels:
      app: sidekiq-worker
  template:
    metadata:
      labels:
        app: sidekiq-worker
    spec:
      containers:
      - name: sidekiq
        image: your-registry/sidekiq-go:latest
        env:
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: redis-secret
              key: url
        - name: RAILS_ENV
          value: "production"
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - "ps aux | grep sidekiq | grep -v grep"
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - "ps aux | grep sidekiq | grep -v grep"
          initialDelaySeconds: 5
          periodSeconds: 5
```

### Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: sidekiq-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: sidekiq-worker
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## Production Best Practices

1. **Redis Configuration**
   - Use managed Redis (AWS ElastiCache, Azure Cache, etc.)
   - Enable persistence (AOF or RDB)
   - Configure proper memory limits
   - Use Redis Sentinel or Cluster for HA

2. **Monitoring**
   - Monitor queue depths
   - Track job processing times
   - Alert on failed jobs
   - Monitor Redis memory usage

3. **Scaling**
   - Scale workers based on queue depth
   - Adjust concurrency per process based on workload
   - Use weighted queues for priority processing

4. **Security**
   - Protect Sidekiq Web UI with authentication
   - Use TLS for Redis connections
   - Restrict network access to Redis
   - Run workers with least privilege

5. **Reliability**
   - Use process supervisors (systemd, k8s)
   - Implement graceful shutdown
   - Monitor and restart failed processes
   - Use rolling deployments

