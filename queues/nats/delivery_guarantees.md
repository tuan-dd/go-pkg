# NATS JetStream Delivery Guarantees Configuration

## Overview
NATS JetStream hỗ trợ 3 loại delivery guarantees:
1. **At-most-once** (fire and forget)
2. **At-least-once** (default)
3. **Exactly-once** (deduplication)

## 1. At-least-once Delivery (Mặc định)

### Cách hoạt động:
- Message sẽ được gửi ít nhất 1 lần
- Có thể bị duplicate nếu consumer crash trước khi Ack
- Phù hợp cho hầu hết use cases

### Cấu hình:
```go
// At-least-once config
options := queue.Options[SubJSOption]{
    Config: SubJSOption{
        Group: "my-consumer-group",
        BasicSOption: BasicSOption{
            AckPolicy:     jetstream.AckExplicitPolicy,  // Bắt buộc phải Ack
            MaxDeliver:    3,                           // Retry tối đa 3 lần
            AckWait:       30 * time.Second,            // Timeout chờ Ack
            DeliverPolicy: jetstream.DeliverAllPolicy,   // Gửi tất cả message
            MaxAckPending: 1000,                        // Số message chưa Ack tối đa
            Delay:         time.Second,                 // Delay giữa các retry
        },
    },
}
```

### Stream Config cho At-least-once:
```go
streamConfig := jetstream.StreamConfig{
    Name:     "MY_STREAM",
    Subjects: []string{"orders.>"},
    Storage:  jetstream.FileStorage,    // Persistent storage
    MaxMsgs:  1000000,
    MaxAge:   24 * time.Hour,
    Replicas: 3,                       // Replication cho HA
    // Không cần Duplicates config
}
```

## 2. Exactly-once Delivery (Deduplication)

### Cách hoạt động:
- Message được gửi đúng 1 lần duy nhất
- NATS sử dụng message ID để detect duplicates
- Phù hợp cho financial transactions, critical operations

### Cấu hình:
```go
// Exactly-once config
options := queue.Options[SubJSOption]{
    Config: SubJSOption{
        Group: "financial-processor",
        BasicSOption: BasicSOption{
            AckPolicy:     jetstream.AckExplicitPolicy,
            MaxDeliver:    1,                           // Chỉ gửi 1 lần
            AckWait:       60 * time.Second,            // Timeout dài hơn
            DeliverPolicy: jetstream.DeliverAllPolicy,
            MaxAckPending: 100,                         // Ít message pending hơn
            Delay:         0,                           // Không retry
        },
    },
}
```

### Stream Config cho Exactly-once:
```go
streamConfig := jetstream.StreamConfig{
    Name:       "FINANCIAL_STREAM",
    Subjects:   []string{"payments.>", "transfers.>"},
    Storage:    jetstream.FileStorage,
    MaxMsgs:    1000000,
    MaxAge:     7 * 24 * time.Hour,
    Replicas:   3,
    Duplicates: 5 * time.Minute,       // Enable deduplication window
}
```

### Publisher cần set Message ID:
```go
msg := &queue.Message{
    ID:   fmt.Sprintf("payment-%s-%d", paymentID, time.Now().UnixNano()),
    Body: paymentData,
}

// NATS sẽ reject duplicate messages với cùng ID trong Duplicates window
```

## 3. Comparison Table

| Feature | At-most-once | At-least-once | Exactly-once |
|---------|-------------|---------------|--------------|
| **Delivery** | 0 hoặc 1 lần | ≥ 1 lần | Đúng 1 lần |
| **Duplicates** | Không | Có thể | Không |
| **Message Loss** | Có thể | Không | Không |
| **Performance** | Cao nhất | Cao | Trung bình |
| **Complexity** | Thấp | Trung bình | Cao |
| **Use Cases** | Logs, metrics | Orders, notifications | Payments, banking |

## 4. Implementation Examples

### At-least-once Example:
```go
// Suitable for order processing
func ProcessOrders() {
    options := queue.Options[SubJSOption]{
        Config: SubJSOption{
            Group: "order-processors",
            BasicSOption: BasicSOption{
                AckPolicy:     jetstream.AckExplicitPolicy,
                MaxDeliver:    5,  // Retry up to 5 times
                AckWait:       30 * time.Second,
                MaxAckPending: 1000,
                Delay:         2 * time.Second,
            },
        },
    }
    
    conn.Subscribe("ORDERS", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
        // Process order (idempotent logic required)
        err := processOrder(msg.Body)
        if err != nil {
            return response.ServerError("processing failed")
        }
        return nil // Auto Ack
    })
}
```

### Exactly-once Example:
```go
// Suitable for financial transactions
func ProcessPayments() {
    // Stream with deduplication
    streamConfig := jetstream.StreamConfig{
        Name:       "PAYMENTS",
        Subjects:   []string{"payments.>"},
        Duplicates: 10 * time.Minute, // 10 minute dedup window
    }
    
    options := queue.Options[SubJSOption]{
        Config: SubJSOption{
            Group: "payment-processors",
            BasicSOption: BasicSOption{
                AckPolicy:     jetstream.AckExplicitPolicy,
                MaxDeliver:    1,  // No retries
                AckWait:       120 * time.Second, // Long timeout
                MaxAckPending: 10, // Low concurrency
            },
        },
    }
    
    conn.Subscribe("PAYMENTS", options, func(ctx context.Context, msg *queue.Message) *response.AppError {
        // Process payment (no need for idempotent logic)
        err := processPayment(msg.Body)
        if err != nil {
            // Log error but don't retry
            logger.Error("Payment processing failed", err)
            return nil // Ack anyway to prevent redelivery
        }
        return nil
    })
}
```

## 5. Best Practices

### For At-least-once:
- ✅ Implement idempotent processing logic
- ✅ Use reasonable retry limits (3-5 times)
- ✅ Handle duplicate detection in business logic
- ✅ Set appropriate AckWait timeouts

### For Exactly-once:
- ✅ Always set unique Message IDs
- ✅ Configure appropriate Duplicates window
- ✅ Handle failures gracefully (don't retry)
- ✅ Use longer AckWait timeouts
- ✅ Monitor deduplication metrics

### General:
- ✅ Use persistent storage (FileStorage) for important data
- ✅ Configure replicas for high availability
- ✅ Monitor MaxAckPending to prevent memory issues
- ✅ Set reasonable MaxAge for message retention
