package notificserver

import (
	"context"
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
		err = emailsend(req.NotificationType)
		if err != nil {
			s.logger.Fatalf("Error: %v", err)
		}
		msg.Ack(false)

	}
	return &pb.NotificationResponse{
		Success: true,
		Message: "OK",
	}, nil
}

func emailsend(nType string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", "blackwinter470@gmail.com")
	m.SetHeader("To", "victor.kohler11@gmail.com")
	m.SetHeader("Subject", "Письмо через gomail")
	switch {
	case nType == "borrow_queue":
		m.SetBody("text/html", `
        <h1>Привет!</h1>

        <p>Это письмо отправлено через библиотеку gomail.</p>
    `)
	}
	d := gomail.NewDialer("smtp.gmail.com", 587, "blackwinter470@gmail.com", "xsou vogt glwz xnfi")

	if err := d.DialAndSend(m); err != nil {
		fmt.Printf("Ошибка отправки: %v\n", err)
		return err
	}

	fmt.Println("Письмо отправлено!")

	return nil
}
