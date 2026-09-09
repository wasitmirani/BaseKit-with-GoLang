package config

// QueueConfig holds message queue configuration
type QueueConfig struct {
	Connection string // sync, redis, database
	RedisQueue string
	RabbitMQ   RabbitMQConfig
	Kafka      KafkaConfig
}

// RabbitMQConfig holds RabbitMQ connection settings
type RabbitMQConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
}

// KafkaConfig holds Kafka connection settings
type KafkaConfig struct {
	Brokers     string
	GroupID     string
	TopicPrefix string
}

func loadQueueConfig() QueueConfig {
	return QueueConfig{
		Connection: getString("QUEUE_CONNECTION", "sync"),
		RedisQueue: getString("REDIS_QUEUE", "default"),
		RabbitMQ: RabbitMQConfig{
			Host:     getString("RABBITMQ_HOST", "127.0.0.1"),
			Port:     getString("RABBITMQ_PORT", "5672"),
			User:     getString("RABBITMQ_USER", "guest"),
			Password: getString("RABBITMQ_PASSWORD", "guest"),
			VHost:    getString("RABBITMQ_VHOST", "/"),
		},
		Kafka: KafkaConfig{
			Brokers:     getString("KAFKA_BROKERS", "127.0.0.1:9092"),
			GroupID:     getString("KAFKA_GROUP_ID", "backoffice-service"),
			TopicPrefix: getString("KAFKA_TOPIC_PREFIX", "backoffice"),
		},
	}
}
