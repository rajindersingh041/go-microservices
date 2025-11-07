# Deployment Guide

This guide covers different deployment strategies for the Go Microservices platform, from local development to production environments.

## 🏠 Local Development Deployment

### Prerequisites
- Docker Desktop (Windows/Mac) or Docker Engine + Docker Compose (Linux)
- Git
- 8GB+ RAM recommended
- Ports 8080-8091, 8123, 9000 available

### Quick Start

```bash
# 1. Clone the repository
git clone <repository-url>
cd go-microservices

# 2. Create environment configuration
cp .env.example .env

# 3. Start all services
docker-compose up -d

# 4. Verify deployment
docker-compose ps
curl http://localhost:8090/query/events
```

### Environment Configuration (.env)

```env
# Database Configuration
CLICKHOUSE_HOST=clickhouse-server
CLICKHOUSE_USER=myuser
CLICKHOUSE_PASSWORD=mypassword
CLICKHOUSE_DB=mydatabase

# Market Poller Configuration
POLLER_INSTRUMENTS=NSE_EQ|INE009A01021,BSE_EQ|500325
UPSTOX_BASE_URL=https://api.upstox.com
POLLER_INGEST_MARKET_URL=http://ingest-marketdata:8081/ingest/marketdata
POLLER_INGEST_EVENTS_URL=http://ingest-events:8080/ingest/events
POLLER_INTERVAL=30s
POLLER_TIMEZONE=Asia/Kolkata
POLLER_START_TIME=09:15
POLLER_END_TIME=15:30

# Optional: Logging
LOG_LEVEL=INFO
LOG_FORMAT=json
```

### Development Workflow

```bash
# Start services in development mode
docker-compose up --build

# View logs from all services
docker-compose logs -f

# View logs from specific service
docker-compose logs -f ingest-events

# Restart specific service after code changes
docker-compose up -d --no-deps --build ingest-events

# Scale a service for testing
docker-compose up --scale query-events=3 -d

# Stop all services
docker-compose down

# Clean up (removes data volumes)
docker-compose down -v
```

---

## 🐋 Docker Deployment

### Single Machine Deployment

#### Option 1: Docker Compose (Recommended for small deployments)

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  # Database
  clickhouse-server:
    image: clickhouse/clickhouse-server:latest
    container_name: clickhouse-prod
    ports:
      - "8123:8123"
      - "9000:9000"
    environment:
      - CLICKHOUSE_DB=production_db
      - CLICKHOUSE_USER=prod_user
      - CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD}
    volumes:
      - clickhouse_data:/var/lib/clickhouse
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "clickhouse-client", "-u", "prod_user", "--password", "${CLICKHOUSE_PASSWORD}", "--query", "SELECT 1"]
      interval: 30s
      timeout: 10s
      retries: 3

  # Ingestion Services
  ingest-events:
    image: your-registry/go-microservices:latest
    command: /bin/ingest-events
    ports:
      - "8080:8080"
    environment:
      - CLICKHOUSE_HOST=clickhouse-server
      - CLICKHOUSE_USER=prod_user
      - CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD}
      - CLICKHOUSE_DB=production_db
    depends_on:
      clickhouse-server:
        condition: service_healthy
    restart: unless-stopped
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M

  ingest-marketdata:
    image: your-registry/go-microservices:latest
    command: /bin/ingest-marketdata
    ports:
      - "8081:8081"
    environment:
      - CLICKHOUSE_HOST=clickhouse-server
      - CLICKHOUSE_USER=prod_user
      - CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD}
      - CLICKHOUSE_DB=production_db
    depends_on:
      clickhouse-server:
        condition: service_healthy
    restart: unless-stopped

  # Query Services (scaled for read load)
  query-events:
    image: your-registry/go-microservices:latest
    command: /bin/query-events
    ports:
      - "8090-8092:8090"
    environment:
      - CLICKHOUSE_HOST=clickhouse-server
      - CLICKHOUSE_USER=prod_user
      - CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD}
      - CLICKHOUSE_DB=production_db
    depends_on:
      clickhouse-server:
        condition: service_healthy
    restart: unless-stopped
    deploy:
      replicas: 3

  query-marketdata:
    image: your-registry/go-microservices:latest
    command: /bin/query-marketdata
    ports:
      - "8091:8091"
    environment:
      - CLICKHOUSE_HOST=clickhouse-server
      - CLICKHOUSE_USER=prod_user
      - CLICKHOUSE_PASSWORD=${CLICKHOUSE_PASSWORD}
      - CLICKHOUSE_DB=production_db
    depends_on:
      clickhouse-server:
        condition: service_healthy
    restart: unless-stopped

  # Background Services
  market-poller:
    image: your-registry/go-microservices:latest
    command: /bin/market-poller
    environment:
      - CLICKHOUSE_HOST=clickhouse-server
      - POLLER_INSTRUMENTS=${POLLER_INSTRUMENTS}
      - UPSTOX_BASE_URL=${UPSTOX_BASE_URL}
      - POLLER_INGEST_MARKET_URL=http://ingest-marketdata:8081/ingest/marketdata
      - POLLER_INGEST_EVENTS_URL=http://ingest-events:8080/ingest/events
      - POLLER_INTERVAL=30s
      - POLLER_TIMEZONE=Asia/Kolkata
      - POLLER_START_TIME=09:15
      - POLLER_END_TIME=15:30
    depends_on:
      - ingest-events
      - ingest-marketdata
    restart: unless-stopped

  # Load Balancer (Optional)
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
      - ./ssl:/etc/nginx/ssl
    depends_on:
      - ingest-events
      - ingest-marketdata
      - query-events
      - query-marketdata
    restart: unless-stopped

volumes:
  clickhouse_data:
    driver: local
```

#### Nginx Load Balancer Configuration

```nginx
# nginx.conf
events {
    worker_connections 1024;
}

http {
    upstream query_events {
        server query-events:8090 max_fails=3 fail_timeout=30s;
        # Add more instances if scaled
    }

    upstream query_marketdata {
        server query-marketdata:8091 max_fails=3 fail_timeout=30s;
    }

    upstream ingest_events {
        server ingest-events:8080 max_fails=3 fail_timeout=30s;
    }

    upstream ingest_marketdata {
        server ingest-marketdata:8081 max_fails=3 fail_timeout=30s;
    }

    server {
        listen 80;
        server_name your-domain.com;

        # Health check endpoint
        location /health {
            access_log off;
            return 200 "healthy\n";
            add_header Content-Type text/plain;
        }

        # Events API
        location /api/events/query {
            proxy_pass http://query_events/query/events;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }

        location /api/events/ingest {
            proxy_pass http://ingest_events/ingest/events;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            client_max_body_size 10M;
        }

        # Market Data API
        location /api/marketdata/query {
            proxy_pass http://query_marketdata/query/marketdata;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }

        location /api/marketdata/ingest {
            proxy_pass http://ingest_marketdata/ingest/marketdata;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            client_max_body_size 50M;
        }
    }
}
```

#### Deployment Commands

```bash
# Build and tag image
docker build -t your-registry/go-microservices:latest .
docker push your-registry/go-microservices:latest

# Deploy with production compose file
docker-compose -f docker-compose.prod.yml up -d

# Update services with zero downtime
docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d --no-deps ingest-events
```

---

## ☸️ Kubernetes Deployment

### Prerequisites
- Kubernetes cluster (1.19+)
- kubectl configured
- Helm (optional, for easier management)

### Namespace Setup

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: go-microservices
---
apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
  namespace: go-microservices
type: Opaque
stringData:
  clickhouse-password: "your-secure-password"
  upstox-api-key: "your-api-key"
```

### ClickHouse Database

```yaml
# clickhouse-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: clickhouse
  namespace: go-microservices
spec:
  replicas: 1
  selector:
    matchLabels:
      app: clickhouse
  template:
    metadata:
      labels:
        app: clickhouse
    spec:
      containers:
      - name: clickhouse
        image: clickhouse/clickhouse-server:latest
        env:
        - name: CLICKHOUSE_DB
          value: "production_db"
        - name: CLICKHOUSE_USER
          value: "prod_user"
        - name: CLICKHOUSE_PASSWORD
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: clickhouse-password
        ports:
        - containerPort: 8123
        - containerPort: 9000
        volumeMounts:
        - name: clickhouse-data
          mountPath: /var/lib/clickhouse
        livenessProbe:
          exec:
            command:
            - clickhouse-client
            - -u
            - prod_user
            - --password
            - $(CLICKHOUSE_PASSWORD)
            - --query
            - SELECT 1
          initialDelaySeconds: 30
          periodSeconds: 30
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "4Gi"
            cpu: "2"
      volumes:
      - name: clickhouse-data
        persistentVolumeClaim:
          claimName: clickhouse-pvc

---
apiVersion: v1
kind: Service
metadata:
  name: clickhouse
  namespace: go-microservices
spec:
  selector:
    app: clickhouse
  ports:
  - name: http
    port: 8123
    targetPort: 8123
  - name: native
    port: 9000
    targetPort: 9000

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: clickhouse-pvc
  namespace: go-microservices
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Gi
```

### Microservices Deployments

```yaml
# microservices-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ingest-events
  namespace: go-microservices
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ingest-events
  template:
    metadata:
      labels:
        app: ingest-events
    spec:
      containers:
      - name: ingest-events
        image: your-registry/go-microservices:latest
        command: ["/bin/ingest-events"]
        env:
        - name: CLICKHOUSE_HOST
          value: "clickhouse"
        - name: CLICKHOUSE_USER
          value: "prod_user"
        - name: CLICKHOUSE_PASSWORD
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: clickhouse-password
        - name: CLICKHOUSE_DB
          value: "production_db"
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"

---
apiVersion: v1
kind: Service
metadata:
  name: ingest-events
  namespace: go-microservices
spec:
  selector:
    app: ingest-events
  ports:
  - port: 8080
    targetPort: 8080

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: query-events
  namespace: go-microservices
spec:
  replicas: 3  # Scaled for read load
  selector:
    matchLabels:
      app: query-events
  template:
    metadata:
      labels:
        app: query-events
    spec:
      containers:
      - name: query-events
        image: your-registry/go-microservices:latest
        command: ["/bin/query-events"]
        env:
        - name: CLICKHOUSE_HOST
          value: "clickhouse"
        - name: CLICKHOUSE_USER
          value: "prod_user"
        - name: CLICKHOUSE_PASSWORD
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: clickhouse-password
        - name: CLICKHOUSE_DB
          value: "production_db"
        ports:
        - containerPort: 8090
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"

---
apiVersion: v1
kind: Service
metadata:
  name: query-events
  namespace: go-microservices
spec:
  selector:
    app: query-events
  ports:
  - port: 8090
    targetPort: 8090
```

### Ingress Configuration

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: microservices-ingress
  namespace: go-microservices
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - api.your-domain.com
    secretName: api-tls
  rules:
  - host: api.your-domain.com
    http:
      paths:
      - path: /events/ingest
        pathType: Prefix
        backend:
          service:
            name: ingest-events
            port:
              number: 8080
      - path: /events/query
        pathType: Prefix
        backend:
          service:
            name: query-events
            port:
              number: 8090
      - path: /marketdata/ingest
        pathType: Prefix
        backend:
          service:
            name: ingest-marketdata
            port:
              number: 8081
      - path: /marketdata/query
        pathType: Prefix
        backend:
          service:
            name: query-marketdata
            port:
              number: 8091
```

### Horizontal Pod Autoscaler

```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: query-events-hpa
  namespace: go-microservices
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: query-events
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Deployment Commands

```bash
# Apply all configurations
kubectl apply -f namespace.yaml
kubectl apply -f clickhouse-deployment.yaml
kubectl apply -f microservices-deployment.yaml
kubectl apply -f ingress.yaml
kubectl apply -f hpa.yaml

# Check deployment status
kubectl get pods -n go-microservices
kubectl get services -n go-microservices
kubectl get ingress -n go-microservices

# View logs
kubectl logs -l app=ingest-events -n go-microservices -f

# Scale deployments
kubectl scale deployment query-events --replicas=5 -n go-microservices
```

---

## ☁️ Cloud Provider Deployments

### AWS EKS Deployment

#### Prerequisites
- AWS CLI configured
- eksctl installed
- kubectl installed

```bash
# Create EKS cluster
eksctl create cluster \
  --name go-microservices-cluster \
  --region us-west-2 \
  --nodegroup-name standard-workers \
  --node-type t3.medium \
  --nodes 3 \
  --nodes-min 1 \
  --nodes-max 4 \
  --managed

# Configure kubectl
aws eks update-kubeconfig --region us-west-2 --name go-microservices-cluster

# Install AWS Load Balancer Controller
eksctl utils associate-iam-oidc-provider --cluster go-microservices-cluster --approve

# Deploy applications (use K8s manifests from above)
kubectl apply -f k8s/
```

#### AWS-specific configurations:

```yaml
# Use EBS for persistent volumes
apiVersion: v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
  fsType: ext4
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer

---
# Update ClickHouse PVC to use EBS
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: clickhouse-pvc
  namespace: go-microservices
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: fast-ssd
  resources:
    requests:
      storage: 100Gi
```

### Google GKE Deployment

```bash
# Create GKE cluster
gcloud container clusters create go-microservices-cluster \
  --zone us-central1-a \
  --machine-type e2-standard-2 \
  --num-nodes 3 \
  --enable-autoscaling \
  --min-nodes 1 \
  --max-nodes 10

# Get credentials
gcloud container clusters get-credentials go-microservices-cluster --zone us-central1-a

# Deploy applications
kubectl apply -f k8s/
```

### Azure AKS Deployment

```bash
# Create resource group
az group create --name go-microservices-rg --location eastus

# Create AKS cluster
az aks create \
  --resource-group go-microservices-rg \
  --name go-microservices-cluster \
  --node-count 3 \
  --enable-addons monitoring \
  --generate-ssh-keys

# Get credentials
az aks get-credentials --resource-group go-microservices-rg --name go-microservices-cluster

# Deploy applications
kubectl apply -f k8s/
```

---

## 📊 Monitoring and Observability

### Prometheus + Grafana Stack

```yaml
# monitoring.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: monitoring

---
# Prometheus
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      containers:
      - name: prometheus
        image: prom/prometheus:latest
        ports:
        - containerPort: 9090
        volumeMounts:
        - name: prometheus-config
          mountPath: /etc/prometheus
        - name: prometheus-data
          mountPath: /prometheus
      volumes:
      - name: prometheus-config
        configMap:
          name: prometheus-config
      - name: prometheus-data
        emptyDir: {}

---
# Grafana
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: grafana
  template:
    metadata:
      labels:
        app: grafana
    spec:
      containers:
      - name: grafana
        image: grafana/grafana:latest
        ports:
        - containerPort: 3000
        env:
        - name: GF_SECURITY_ADMIN_PASSWORD
          value: "admin123"
```

### Application Metrics

Add to your Go services:

```go
// Add to main.go of each service
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )
    
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request duration",
        },
        []string{"method", "endpoint"},
    )
)

func init() {
    prometheus.MustRegister(requestsTotal)
    prometheus.MustRegister(requestDuration)
}

func main() {
    // Add metrics endpoint
    http.Handle("/metrics", promhttp.Handler())
    
    // Wrap handlers with metrics
    http.HandleFunc("/ingest/events", metricsMiddleware(app.handleIngest))
}

func metricsMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        next(w, r)
        
        duration := time.Since(start).Seconds()
        requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
        requestsTotal.WithLabelValues(r.Method, r.URL.Path, "200").Inc()
    }
}
```

---

## 🔒 Security Considerations

### TLS/SSL Configuration

```yaml
# tls-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: tls-secret
  namespace: go-microservices
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-cert>
  tls.key: <base64-encoded-key>
```

### Network Policies

```yaml
# network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: microservices-netpol
  namespace: go-microservices
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: go-microservices
    - podSelector: {}
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: go-microservices
    - podSelector: {}
  - to: []
    ports:
    - protocol: TCP
      port: 53
    - protocol: UDP
      port: 53
```

### RBAC Configuration

```yaml
# rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: microservices-sa
  namespace: go-microservices

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: microservices-role
  namespace: go-microservices
rules:
- apiGroups: [""]
  resources: ["pods", "services"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: microservices-binding
  namespace: go-microservices
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: microservices-role
subjects:
- kind: ServiceAccount
  name: microservices-sa
  namespace: go-microservices
```

---

## 🚀 CI/CD Pipeline

### GitHub Actions Example

```yaml
# .github/workflows/deploy.yml
name: Deploy Microservices

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.25
    
    - name: Run tests
      run: go test ./...
    
    - name: Build and push Docker image
      uses: docker/build-push-action@v3
      with:
        context: .
        push: true
        tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ github.sha }}
    
  deploy:
    needs: build
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Configure kubectl
      uses: azure/setup-kubectl@v3
    
    - name: Deploy to Kubernetes
      run: |
        kubectl set image deployment/ingest-events ingest-events=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ github.sha }} -n go-microservices
        kubectl set image deployment/query-events query-events=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ github.sha }} -n go-microservices
        kubectl rollout status deployment/ingest-events -n go-microservices
        kubectl rollout status deployment/query-events -n go-microservices
```

This deployment guide provides comprehensive coverage of different deployment strategies, from local development to production cloud deployments, ensuring your microservices platform can scale from proof-of-concept to enterprise production workloads.