package userclient

import (
	"context"
	"time"

	pb "github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type UserClient struct {
	client  pb.UserServiceClient
	timeout time.Duration
	conn    *grpc.ClientConn
	logger  *logrus.Logger
}

func NewUserClient(addr string, timeout time.Duration, logger *logrus.Logger) (*UserClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return &UserClient{
		conn:    conn,
		timeout: timeout,
		client:  pb.NewUserServiceClient(conn),
		logger:  logger,
	}, nil
}

func (c *UserClient) Create(ctx context.Context, name, email string) (*pb.UserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.CreateUser(ctx, &pb.CreateUserRequest{
		Name:  name,
		Email: email,
	})
}

func (c *UserClient) Get(ctx context.Context, id string) (*pb.UserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if id == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "UserId cannot be empty")
	}
	c.logger.Infof("GetUser called with UserId: %s", id)
	return c.client.GetUser(ctx, &pb.GetUserRequest{
		UserId: id,
	})

}

func (c *UserClient) Close() error {
	return c.conn.Close()
}
