package userclient

import (
	"context"
	"net/mail"
	"time"

	pb "github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type UserClient struct {
	client  pb.UserServiceClient
	timeout time.Duration
	conn    *grpc.ClientConn
	logger  *logrus.Logger
}

func NewUserClient(addr string, timeout time.Duration, logger *logrus.Logger) (*UserClient, error) {
	conn, err := grpc.Dial("localhost:"+addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	err := validateCreateUserRequest(name, email)
	if err != nil {
		return nil, err
	}
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

func validateCreateUserRequest(name, email string) error {
	if name == "" {
		return status.Error(codes.InvalidArgument, "name cannot be empty")
	}
	if email == "" || !isValidEmail(email) {
		return status.Error(codes.InvalidArgument, "invalid email")
	}
	return nil
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}
