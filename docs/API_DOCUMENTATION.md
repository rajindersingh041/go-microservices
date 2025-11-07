# API Documentation

## Overview

This document provides comprehensive API documentation for all microservices in the Go Microservices platform. Each service exposes RESTful HTTP endpoints for specific business domains.

## Service Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Port 8080     │    │   Port 8081     │    │   Port 8090     │
│ Events Ingest   │    │ Market Ingest   │    │ Events Query    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                   ┌─────────────────┐
                   │   Port 8091     │
                   │ Market Query    │
                   └─────────────────┘
```

## Common Response Codes

| Code | Meaning | Description |
|------|---------|-------------|
| 200 | OK | Request successful, data returned |
| 202 | Accepted | Request accepted for processing |
| 400 | Bad Request | Invalid request format or parameters |
| 405 | Method Not Allowed | HTTP method not supported for endpoint |
| 500 | Internal Server Error | Server-side error occurred |

## Content Types

All APIs accept and return JSON:
- **Request**: `Content-Type: application/json`
- **Response**: `Content-Type: application/json`

---

# Events Service APIs

## Event Ingestion Service (Port 8080)

### POST /ingest/events

**Purpose**: Store application events, logs, and audit trails

**Endpoint**: `http://localhost:8080/ingest/events`

**Method**: `POST`

**Content-Type**: `application/json`

#### Request Body

**Single Event**:
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "source": "user-service",
  "message": "User login successful",
  "context": {
    "userId": "12345",
    "sessionId": "abc-def-ghi",
    "ipAddress": "192.168.1.100"
  }
}
```

**Multiple Events (Batch)**:
```json
[
  {
    "timestamp": "2024-01-15T10:30:00Z",
    "level": "INFO",
    "source": "user-service",
    "message": "User login successful",
    "context": {
      "userId": "12345"
    }
  },
  {
    "timestamp": "2024-01-15T10:31:00Z",
    "level": "ERROR",
    "source": "payment-service",
    "message": "Payment processing failed",
    "context": {
      "orderId": "ORD-789",
      "amount": "99.99",
      "currency": "USD",
      "errorCode": "INSUFFICIENT_FUNDS"
    }
  }
]
```

#### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `timestamp` | string (ISO 8601) | Yes | When the event occurred |
| `level` | string | Yes | Event severity: INFO, WARN, ERROR, DEBUG |
| `source` | string | Yes | Service/component that generated the event |
| `message` | string | Yes | Human-readable event description |
| `context` | object | No | Key-value pairs with additional event data |

#### Example Responses

**Success (202 Accepted)**:
```
HTTP/1.1 202 Accepted
Content-Length: 0
```

**Error (400 Bad Request)**:
```json
{
  "error": "Invalid JSON format",
  "details": "Expected array or object"
}
```

#### cURL Examples

```bash
# Single event
curl -X POST http://localhost:8080/ingest/events \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": "2024-01-15T10:30:00Z",
    "level": "INFO",
    "source": "api-gateway",
    "message": "Request rate limit applied",
    "context": {
      "clientId": "client-123",
      "endpoint": "/api/users",
      "rateLimit": "100/hour"
    }
  }'

# Multiple events
curl -X POST http://localhost:8080/ingest/events \
  -H "Content-Type: application/json" \
  -d '[
    {
      "timestamp": "2024-01-15T10:30:00Z",
      "level": "WARN",
      "source": "inventory-service",
      "message": "Low stock alert",
      "context": {
        "productId": "PROD-456",
        "currentStock": "3",
        "threshold": "10"
      }
    }
  ]'
```

---

## Event Query Service (Port 8090)

### GET /query/events

**Purpose**: Retrieve stored events for monitoring, debugging, and analysis

**Endpoint**: `http://localhost:8090/query/events`

**Method**: `GET`

**Parameters**: None (returns latest 10 events)

#### Response Format

```json
[
  {
    "timestamp": "2024-01-15T10:30:00Z",
    "level": "INFO",
    "source": "user-service",
    "message": "User login successful",
    "context": {
      "userId": "12345",
      "sessionId": "abc-def-ghi"
    }
  },
  {
    "timestamp": "2024-01-15T10:29:30Z",
    "level": "ERROR",
    "source": "payment-service",
    "message": "Database connection timeout",
    "context": {
      "retryCount": "3",
      "timeout": "5000ms"
    }
  }
]
```

#### cURL Example

```bash
# Get recent events
curl http://localhost:8090/query/events

# Pretty print JSON response
curl http://localhost:8090/query/events | jq '.'
```

#### Use Cases

- **Real-time monitoring**: Check latest system events
- **Debugging**: Find error events for troubleshooting
- **Audit trails**: Review user actions and system changes
- **Analytics**: Process events for business insights

---

# Market Data Service APIs

## Market Data Ingestion Service (Port 8081)

### POST /ingest/marketdata

**Purpose**: Store real-time market data from external financial APIs

**Endpoint**: `http://localhost:8081/ingest/marketdata`

**Method**: `POST`

**Content-Type**: `application/json`

#### Request Body

The service expects data in the Upstox API response format:

```json
{
  "data": {
    "request_id": "upstox-req-12345",
    "time_in_millis": 1640995800000,
    "token_data": {
      "NSE_EQ|INE009A01021": {
        "timestamp": "1640995800",
        "lastTradeTime": "1640995795",
        "lastPrice": 2750.50,
        "closePrice": 2745.00,
        "lastQuantity": 100,
        "buyQuantity": 15000.0,
        "sellQuantity": 12000.0,
        "volume": 1500000,
        "averagePrice": 2748.25,
        "oi": 0.0,
        "poi": 0.0,
        "oiDayHigh": 0.0,
        "oiDayLow": 0.0,
        "netChange": 5.50,
        "lowerCircuitLimit": 2470.50,
        "upperCircuitLimit": 3020.50,
        "yl": 2200.00,
        "yh": 3100.00,
        "ohlc": {
          "open": 2740.00,
          "high": 2755.00,
          "low": 2735.00,
          "close": 2750.50,
          "volume": 1500000
        }
      },
      "BSE_EQ|500325": {
        "timestamp": "1640995800",
        "lastPrice": 2751.00,
        "volume": 800000
      }
    }
  },
  "success": true
}
```

#### Field Descriptions

**Root Level**:
| Field | Type | Description |
|-------|------|-------------|
| `data` | object | Container for market data |
| `success` | boolean | API response status |

**Data Level**:
| Field | Type | Description |
|-------|------|-------------|
| `request_id` | string | Unique identifier for the API request |
| `time_in_millis` | number | Response timestamp in milliseconds |
| `token_data` | object | Map of instrument symbols to their data |

**Token Data** (per instrument):
| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string | Data timestamp (Unix) |
| `lastTradeTime` | string | Last trade timestamp |
| `lastPrice` | number | Current market price |
| `closePrice` | number | Previous day's closing price |
| `lastQuantity` | number | Quantity of last trade |
| `buyQuantity` | number | Total buy-side quantity |
| `sellQuantity` | number | Total sell-side quantity |
| `volume` | number | Total volume traded |
| `averagePrice` | number | Volume-weighted average price |
| `netChange` | number | Price change from previous close |
| `ohlc` | object | Open, High, Low, Close data |

#### Response

**Success (202 Accepted)**:
```
HTTP/1.1 202 Accepted
Content-Length: 0
```

#### cURL Example

```bash
curl -X POST http://localhost:8081/ingest/marketdata \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "request_id": "test-request-123",
      "time_in_millis": 1640995800000,
      "token_data": {
        "AAPL": {
          "timestamp": "1640995800",
          "lastPrice": 150.25,
          "closePrice": 149.50,
          "volume": 1000000,
          "netChange": 0.75,
          "ohlc": {
            "open": 149.00,
            "high": 151.00,
            "low": 148.50,
            "close": 150.25,
            "volume": 1000000
          }
        }
      }
    },
    "success": true
  }'
```

---

## Market Data Query Service (Port 8091)

### GET /query/marketdata

**Purpose**: Retrieve stored market data for analysis and reporting

**Endpoint**: `http://localhost:8091/query/marketdata`

**Method**: `GET`

**Parameters**: None (returns latest market data)

#### Response Format

```json
[
  {
    "RequestID": "upstox-req-12345",
    "ResponseTime": "2024-01-15T10:30:00.123Z",
    "TokenName": "NSE_EQ|INE009A01021",
    "Timestamp": "2024-01-15T10:30:00Z",
    "LastTradeTime": "2024-01-15T10:29:55Z",
    "LastPrice": 2750.50,
    "ClosePrice": 2745.00,
    "Volume": 1500000,
    "NetChange": 5.50,
    "OhlcOpen": 2740.00,
    "OhlcHigh": 2755.00,
    "OhlcLow": 2735.00,
    "OhlcClose": 2750.50
  }
]
```

#### cURL Example

```bash
# Get recent market data
curl http://localhost:8091/query/marketdata

# Filter and format with jq
curl http://localhost:8091/query/marketdata | \
  jq '.[] | {symbol: .TokenName, price: .LastPrice, change: .NetChange}'
```

---

# Background Services

## Market Poller Service

**Purpose**: Automatically fetch market data from external APIs during market hours

**Type**: Background service (no HTTP endpoints)

**Configuration**: Via environment variables

### Environment Variables

```env
# Instruments to poll (comma-separated)
POLLER_INSTRUMENTS=NSE_EQ|INE009A01021,BSE_EQ|500325

# External API configuration
UPSTOX_BASE_URL=https://api.upstox.com

# Internal service URLs
POLLER_INGEST_MARKET_URL=http://ingest-marketdata:8081/ingest/marketdata
POLLER_INGEST_EVENTS_URL=http://ingest-events:8080/ingest/events

# Polling configuration
POLLER_INTERVAL=30s
POLLER_TIMEZONE=Asia/Kolkata
POLLER_START_TIME=09:15
POLLER_END_TIME=15:30
```

### Behavior

1. **Market Hours Check**: Only polls during configured market hours
2. **Periodic Fetching**: Polls external API at configured intervals
3. **Data Forwarding**: Sends market data to ingestion service
4. **Event Logging**: Records polling activities and errors
5. **Error Handling**: Continues operation despite temporary failures

### Logs

```
2024-01-15 10:30:00 INFO Starting Market Poller Service
2024-01-15 10:30:00 INFO Loaded 2 instruments. Fetching every 30s.
2024-01-15 10:30:00 INFO Time window: 09:15-15:30 (Asia/Kolkata).
2024-01-15 10:30:30 INFO Market is open. Fetching data for 2 instruments.
2024-01-15 10:30:31 INFO Successfully forwarded market data to ingest service
```

---

# Error Handling

## Common Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid JSON format",
  "details": "Expected object or array",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### 405 Method Not Allowed
```json
{
  "error": "Method not allowed",
  "allowed_methods": ["POST"],
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### 500 Internal Server Error
```json
{
  "error": "Database connection failed",
  "request_id": "req-12345",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Retry Logic

For ingestion endpoints, implement exponential backoff:

```bash
# Retry with backoff
for i in {1..3}; do
  if curl -f -X POST http://localhost:8080/ingest/events -d "$data"; then
    break
  else
    sleep $((2**i))
  fi
done
```

---

# Integration Examples

## Complete Data Flow Example

```bash
#!/bin/bash

# 1. Ingest some events
echo "Ingesting events..."
curl -X POST http://localhost:8080/ingest/events \
  -H "Content-Type: application/json" \
  -d '[{
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "level": "INFO",
    "source": "integration-test",
    "message": "Test event created",
    "context": {"testId": "123"}
  }]'

# 2. Ingest market data
echo "Ingesting market data..."
curl -X POST http://localhost:8081/ingest/marketdata \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "request_id": "test-123",
      "time_in_millis": '$(date +%s000)',
      "token_data": {
        "TEST_SYMBOL": {
          "timestamp": "'$(date +%s)'",
          "lastPrice": 100.50,
          "volume": 1000
        }
      }
    },
    "success": true
  }'

# 3. Query the data back
echo "Querying events..."
curl http://localhost:8090/query/events | jq '.[0]'

echo "Querying market data..."
curl http://localhost:8091/query/marketdata | jq '.[0]'
```

## Monitoring Integration

```bash
# Health check all services
services=("8080" "8081" "8090" "8091")
for port in "${services[@]}"; do
  if curl -f "http://localhost:$port/health" 2>/dev/null; then
    echo "Service on port $port: UP"
  else
    echo "Service on port $port: DOWN"
  fi
done
```

## Load Testing

```bash
# Install Apache Bench
apt-get install apache2-utils

# Test event ingestion
ab -n 1000 -c 10 -p event.json -T application/json \
   http://localhost:8080/ingest/events

# Test event querying  
ab -n 1000 -c 10 http://localhost:8090/query/events
```

---

# API Versioning Strategy

For production deployments, implement API versioning:

```
/v1/ingest/events    # Version 1 (current)
/v2/ingest/events    # Version 2 (future)
```

## Backward Compatibility

- Never remove required fields
- Add optional fields only
- Maintain field name consistency
- Document breaking changes clearly

## Client Libraries

Consider providing client libraries for common languages:

```go
// Go client
client := microservices.NewClient("http://localhost:8080")
err := client.IngestEvents([]Event{...})
```

```python
# Python client
from microservices import Client
client = Client("http://localhost:8080")
client.ingest_events([{...}])
```

This completes the comprehensive API documentation for the Go Microservices platform.