package notificlient

import (
	"context"
	"time"

	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NotificClient struct {
	client  pb.NotificationServiceClient
	timeout time.Duration
	conn    *grpc.ClientConn
	logger  *logrus.Logger
}

func NewNotificationClient(addr string, timeout time.Duration, logger *logrus.Logger) (*NotificClient, error) {
	conn, err := grpc.Dial("localhost:"+addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &NotificClient{
		conn:    conn,
		timeout: timeout,
		client:  pb.NewNotificationServiceClient(conn),
		logger:  logger,
	}, nil
}

func (c *NotificClient) Send(ctx context.Context, nType string) (*pb.NotificationResponse, error) {
	return c.client.SendNotification(ctx, &pb.NotificationRequest{
		NotificationType: nType,
	},
	)
}

func (c *NotificClient) Close() error {
	return c.conn.Close()
}
