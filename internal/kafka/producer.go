package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"go.uber.org/zap"
)

// Producer wraps a confluent-kafka-go producer with structured logging and
// graceful shutdown.
type Producer struct {
	producer *kafka.Producer
	logger   *zap.Logger
}

// ProducerConfig holds the configuration needed to create a Producer.
type ProducerConfig struct {
	// ConfigMap accepts the same key/value pairs as confluent-kafka-go's
	// kafka.ConfigMap. Using map[string]any avoids importing the kafka package
	// outside of this package.
	ConfigMap map[string]any
	Logger    *zap.Logger
}

// NewProducer creates a new Kafka producer.
func NewProducer(cfg ProducerConfig) (*Producer, error) {
	cm := make(kafka.ConfigMap, len(cfg.ConfigMap))
	for k, v := range cfg.ConfigMap {
		cm[k] = v
	}
	producer, err := kafka.NewProducer(&cm)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	p := &Producer{
		producer: producer,
		logger:   cfg.Logger,
	}

	return p, nil
}

// Produce sends a message to the specified topic and waits for delivery
// confirmation or context cancellation.
func (p *Producer) Produce(ctx context.Context, topic string, key, value []byte, headers map[string]string) error {
	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            key,
		Value:          value,
		Timestamp:      time.Now(),
	}

	if len(headers) > 0 {
		msg.Headers = make([]kafka.Header, 0, len(headers))
		for k, v := range headers {
			msg.Headers = append(msg.Headers, kafka.Header{Key: k, Value: []byte(v)})
		}
	}

	kafkaChan := make(chan kafka.Event, 1)
	if err := p.producer.Produce(msg, kafkaChan); err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}

	select {
	case e := <-kafkaChan:
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				return fmt.Errorf("message delivery failed: %w", ev.TopicPartition.Error)
			}
			return nil
		case kafka.Error:
			return fmt.Errorf("kafka error: %w", ev)
		default:
			return fmt.Errorf("unexpected event type: %T", e)
		}
	case <-ctx.Done():
		return fmt.Errorf("produce cancelled: %w", ctx.Err())
	}
}

// Close flushes pending messages and closes the underlying producer.
func (p *Producer) Close() {
	p.producer.Flush(5000)
	p.producer.Close()
}

// IsConnected performs a lightweight metadata fetch to verify the broker is
// reachable. A 3-second timeout is used so that the /ready probe fails quickly
// when Kafka is unavailable rather than hanging indefinitely.
func (p *Producer) IsConnected() bool {
	if p.producer == nil {
		return false
	}
	// GetMetadata with allTopics=false fetches only broker-level metadata.
	// A successful call proves the TCP connection to at least one broker is live.
	_, err := p.producer.GetMetadata(nil, false, 3000)
	return err == nil
}
