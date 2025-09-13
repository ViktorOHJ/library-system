package rabbit

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type RabbitMQClient struct {
	conn *amqp091.Connection
	ch   *amqp091.Channel
}

type TaskMessage struct {
	Type       string `json:"type"`
	UserName   string `json:"user_name"`
	BookTitle  string `json:"book_title"`
	BookAuthor string `json:"book_author"`
	DueDate    string `json:"due_date"`
	LoanID     string `json:"loan_id"`
	Email      string `json:"email"`
}

func NewRabbitMQClient(logger *logrus.Logger, url string) (*RabbitMQClient, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		logger.Errorf("Error to connect RabbitMQ: %v", err)
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		logger.Errorf("Error to open channel: %v", err)
		return nil, err
	}
	return &RabbitMQClient{conn: conn, ch: ch}, nil
}

func (r *RabbitMQClient) PublishTask(parentCtx context.Context, logger *logrus.Logger, message *TaskMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		logger.Errorf("Marshaling error: %v", err)
		return err
	}

	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
	defer cancel()

	switch {
	case message.Type == "Borrow":
		logger.Infof("message boorow: %v", message)
		err = r.ch.PublishWithContext(
			ctx,
			"library",
			"borrow_queue",
			false, // mandatory
			false, // immediate
			amqp091.Publishing{
				ContentType: "application/json",
				Body:        body,
				Timestamp:   time.Now(),
			},
		)
	case message.Type == "Return":
		logger.Infof("message return: %v", message)
		err = r.ch.PublishWithContext(
			ctx,
			"library",
			"return_queue",
			false, // mandatory
			false, // immediate
			amqp091.Publishing{
				ContentType: "application/json",
				Body:        body,
				Timestamp:   time.Now(),
			},
		)
	default:
		return errors.New("invalid queue")
	}

	if err != nil {
		logger.Errorf("Publish Error: %v", err)
		return err
	}
	return nil
}

func (r *RabbitMQClient) ConsumeFromQueue(queueName string) (<-chan amqp091.Delivery, error) {
	return r.ch.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack (ручное подтверждение)
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
}

func (r *RabbitMQClient) Close() {
	r.ch.Close()
	r.conn.Close()
}
