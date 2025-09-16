package notificserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/ViktorOHJ/library-system/rabbit"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/gomail.v2"
)

type NotificServer struct {
	pb.UnimplementedNotificationServiceServer
	logger *logrus.Logger
}

func NewNotificServer(logger *logrus.Logger) *NotificServer {
	return &NotificServer{
		logger: logger,
	}
}

func (s *NotificServer) SendNotification(ctx context.Context, req *pb.NotificationRequest) (res *pb.NotificationResponse, err error) {
	s.logger.Info("SendNotification called")

	rabbitURL := os.Getenv("RABBIT_URL")
	rabbit, err := rabbit.NewRabbitMQClient(s.logger, rabbitURL)
	if err != nil {
		s.logger.Errorf("Error to create rabbit client: %v", err)
		return nil, status.Errorf(codes.Internal, "Server Error")
	}
	defer rabbit.Close()

	msgs, err := rabbit.ConsumeFromQueue(req.NotificationType)
	if err != nil {
		s.logger.Errorf("Error to consume messages: %v", err)
		return nil, status.Errorf(codes.Internal, "Server Error")
	}
	for msg := range msgs {
		s.logger.Infof("Received: %s", msg.Body)
		err = sendEmail(s.logger, msg.Body)
		if err != nil {
			s.logger.Errorf("Error: %v", err)
		}
		msg.Ack(false)

	}
	return &pb.NotificationResponse{
		Success: true,
	}, nil
}

func sendEmail(logger *logrus.Logger, message []byte) error {
	var event rabbit.TaskMessage
	err := json.Unmarshal([]byte(message), &event)
	if err != nil {
		return err
	}
	if event.Email == "" {
		logger.Error("Email is empty")
		return fmt.Errorf("email is empty")
	}
	email := os.Getenv("EMAIL")
	mailPass := os.Getenv("MAIL_PASS")
	if email == "" {
		logger.Error("EMAIL environment variable is not set")
		return fmt.Errorf("EMAIL environment variable is not set")
	}
	if mailPass == "" {
		logger.Error("MAIL_PASS environment variable is not set")
		return fmt.Errorf("MAIL_PASS environment variable is not set")
	}
	var emailMessage []byte
	m := gomail.NewMessage()
	m.SetHeader("From", email)
	m.SetHeader("To", event.Email)
	switch event.Type {
	case "Borrow":
		m.SetHeader("Subject", "Book Borrowed Notification")
		emailMessage = []byte(fmt.Sprintf("Dear %s,\n\nYou have successfully borrowed the book \"%s\" by %s. Please remember to return it by %s.\n\nHappy reading!\nLibrary Team",
			event.UserName, event.BookTitle, event.BookAuthor, event.DueDate))
	case "Return":
		m.SetHeader("Subject", "Book Return Confirmation")
		emailMessage = []byte(fmt.Sprintf("Dear %s,\n\nThank you for returning the book \"%s\" by %s. We hope you enjoyed reading it!\n\nBest regards,\nLibrary Team",
			event.UserName, event.BookTitle, event.BookAuthor))
	default:
		logger.Error("Unknown event type")
		return fmt.Errorf("unknown event type")
	}
	m.SetBody("text/html", string(emailMessage))
	d := gomail.NewDialer("smtp.gmail.com", 465, email, mailPass)
	d.SSL = true

	if err := d.DialAndSend(m); err != nil {
		logger.Errorf("Sending error: %v\n", err)
		return err
	}

	logger.Info("Email sent successfully")

	return nil
}
