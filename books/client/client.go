package bookclient

import (
	"context"
	"time"

	pb "github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type BookClient struct {
	client  pb.BookServiceClient
	timeout time.Duration
	conn    *grpc.ClientConn
	logger  *logrus.Logger
}

func NewBookClient(addr string, timeout time.Duration, logger *logrus.Logger) (*BookClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return &BookClient{
		conn:    conn,
		timeout: timeout,
		client:  pb.NewBookServiceClient(conn),
	}, nil
}

func (c *BookClient) Create(ctx context.Context, title, author string, year int32) (*pb.BookResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.CreateBook(ctx, &pb.CreateBookRequest{
		Title:  title,
		Author: author,
		Year:   year,
	})
}

func (c *BookClient) Get(ctx context.Context, id string) (*pb.BookResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if id == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "BookId cannot be empty")
	}
	c.logger.Infof("GetBook called with BookId: %s", id)
	return c.client.GetBook(ctx, &pb.GetBookRequest{
		BookId: id,
	})
}

func (c *BookClient) Update(ctx context.Context, id string) (*pb.BookResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if id == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "BookId cannot be empty")
	}
	return c.client.UpdateBookStatus(ctx, &pb.UpdateBookRequest{
		BookId: id,
	})
}

func (c *BookClient) Close() error {
	return c.conn.Close()
}
