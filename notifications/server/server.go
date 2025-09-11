package notificserver

import (
	"context"
	"os"
	"time"

	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/ViktorOHJ/library-system/rabbit"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		time.Sleep(3 * time.Second)
		msg.Ack(false)

	}
	return &pb.NotificationResponse{
		Success: true,
		Message: "OK",
	}, nil

}
