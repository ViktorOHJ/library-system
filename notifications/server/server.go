package notificserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/ViktorOHJ/library-system/rabbit"
	"github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/gomail.v2"
)

type EmailSender interface {
	SendEmail(to, subject, body string) error
}

type MessageConsumer interface {
	ConsumeFromQueue(queueName string) (<-chan amqp091.Delivery, error)
	Close()
}

type SMTPEmailSender struct {
	host     string
	port     int
	username string
	password string
}

func NewSMTPEmailSender(host string, port int, username, password string) *SMTPEmailSender {
	return &SMTPEmailSender{
		host:     host,
		port:     port,
		username: username,
		password: password,
	}
}

func (s *SMTPEmailSender) SendEmail(to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.username)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(s.host, s.port, s.username, s.password)
	d.SSL = true

	return d.DialAndSend(m)
}

type RabbitConsumer struct {
	client *rabbit.RabbitMQClient
}

func NewRabbitConsumer(rabbitURL string, logger *logrus.Logger) (*RabbitConsumer, error) {
	client, err := rabbit.NewRabbitMQClient(logger, rabbitURL)
	if err != nil {
		return nil, err
	}
	return &RabbitConsumer{client: client}, nil
}

func (r *RabbitConsumer) ConsumeFromQueue(queueName string) (<-chan amqp091.Delivery, error) {
	return r.client.ConsumeFromQueue(queueName)
}

func (r *RabbitConsumer) Close() {
	r.client.Close()
}

type NotificServer struct {
	pb.UnimplementedNotificationServiceServer
	logger          *logrus.Logger
	emailSender     EmailSender
	messageConsumer MessageConsumer
}

func NewNotificServer(logger *logrus.Logger) *NotificServer {
	return &NotificServer{
		logger: logger,
	}
}

func NewNotificServerWithDeps(logger *logrus.Logger, emailSender EmailSender, messageConsumer MessageConsumer) *NotificServer {
	return &NotificServer{
		logger:          logger,
		emailSender:     emailSender,
		messageConsumer: messageConsumer,
	}
}

func (s *NotificServer) SendNotification(ctx context.Context, req *pb.NotificationRequest) (*pb.NotificationResponse, error) {
	s.logger.Info("SendNotification called")

	if err := s.initDependencies(); err != nil {
		s.logger.Errorf("Failed to initialize dependencies: %v", err)
		return nil, status.Errorf(codes.Internal, "Server Error")
	}

	msgs, err := s.messageConsumer.ConsumeFromQueue(req.NotificationType)
	if err != nil {
		s.logger.Errorf("Error consuming messages: %v", err)
		return nil, status.Errorf(codes.Internal, "Server Error")
	}

	for msg := range msgs {
		s.logger.Infof("Received: %s", msg.Body)

		if err := s.processMessage(msg.Body); err != nil {
			s.logger.Errorf("Error processing message: %v", err)
			msg.Nack(false, true)
			continue
		}

		msg.Ack(false)
	}

	return &pb.NotificationResponse{Success: true}, nil
}

func (s *NotificServer) processMessage(messageBody []byte) error {
	var event rabbit.TaskMessage
	if err := json.Unmarshal(messageBody, &event); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	if event.Email == "" {
		return fmt.Errorf("email is empty")
	}

	subject, body := s.formatEmailContent(event)

	return s.emailSender.SendEmail(event.Email, subject, body)
}

func (s *NotificServer) formatEmailContent(event rabbit.TaskMessage) (subject, body string) {
	switch event.Type {
	case "Borrow":
		subject = "Book Borrowed Notification"
		body = fmt.Sprintf(`
			<html>
			<body>
			<h2>Book Borrowed Successfully!</h2>
			<p>Dear %s,</p>
			<p>You have successfully borrowed the book <strong>"%s"</strong> by %s.</p>
			<p>Please remember to return it by <strong>%s</strong>.</p>
			<p>Happy reading!</p>
			<p>Library Team</p>
			</body>
			</html>
		`, event.UserName, event.BookTitle, event.BookAuthor, event.DueDate)
	case "Return":
		subject = "Book Return Confirmation"
		body = fmt.Sprintf(`
			<html>
			<body>
			<h2>Book Returned Successfully!</h2>
			<p>Dear %s,</p>
			<p>Thank you for returning the book <strong>"%s"</strong> by %s.</p>
			<p>We hope you enjoyed reading it!</p>
			<p>Best regards,</p>
			<p>Library Team</p>
			</body>
			</html>
		`, event.UserName, event.BookTitle, event.BookAuthor)
	default:
		subject = "Library Notification"
		body = "<p>Unknown notification type</p>"
	}
	return subject, body
}

func (s *NotificServer) initDependencies() error {
	if s.emailSender == nil {
		email := os.Getenv("EMAIL")
		mailPass := os.Getenv("MAIL_PASS")

		if email == "" || mailPass == "" {
			return fmt.Errorf("EMAIL or MAIL_PASS environment variables not set")
		}

		s.emailSender = NewSMTPEmailSender("smtp.gmail.com", 465, email, mailPass)
	}

	if s.messageConsumer == nil {
		rabbitURL := os.Getenv("RABBIT_URL")
		if rabbitURL == "" {
			return fmt.Errorf("RABBIT_URL environment variable not set")
		}

		consumer, err := NewRabbitConsumer(rabbitURL, s.logger)
		if err != nil {
			return fmt.Errorf("failed to create rabbit consumer: %w", err)
		}

		s.messageConsumer = consumer
	}

	return nil
}

func (s *NotificServer) ValidateConfig() error {
	requiredEnvVars := []string{"EMAIL", "MAIL_PASS", "RABBIT_URL"}

	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			return fmt.Errorf("%s environment variable is not set", envVar)
		}
	}

	return nil
}

func (s *NotificServer) Shutdown() {
	if s.messageConsumer != nil {
		s.messageConsumer.Close()
	}
}
