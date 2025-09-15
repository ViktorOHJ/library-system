package bookclient

import (
	"context"
	"strconv"
	"time"

	pb "github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type BookClient struct {
	client  pb.BookServiceClient
	timeout time.Duration
	conn    *grpc.ClientConn
	logger  *logrus.Logger
}

func NewBookClient(addr string, timeout time.Duration, logger *logrus.Logger) (*BookClient, error) {
	conn, err := grpc.Dial("localhost:"+addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &BookClient{
		conn:    conn,
		timeout: timeout,
		client:  pb.NewBookServiceClient(conn),
		logger:  logger,
	}, nil
}

func (c *BookClient) Create(ctx context.Context, title, author string, year int32) (*pb.BookResponse, error) {
	if title == "" {
		return nil, status.Error(codes.InvalidArgument, "title cannot be empty")
	}
	if len(title) > 255 {
		return nil, status.Error(codes.InvalidArgument, "title too long")
	}
	if author == "" {
		return nil, status.Error(codes.InvalidArgument, "author cannot be empty")
	}
	if len(author) > 100 {
		return nil, status.Error(codes.InvalidArgument, "author name too long")
	}
	if year < 0 || year > int32(time.Now().Year()+10) {
		return nil, status.Error(codes.InvalidArgument, "invalid year")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	c.logger.WithFields(logrus.Fields{
		"title":  title,
		"author": author,
		"year":   year,
	}).Info("Creating book")

	resp, err := c.client.CreateBook(ctx, &pb.CreateBookRequest{
		Title:  title,
		Author: author,
		Year:   year,
	})

	if err != nil {
		c.logger.WithError(err).Error("Failed to create book")
		return nil, err
	}

	c.logger.WithField("book_id", resp.Id).Info("Book created successfully")
	return resp, nil
}

func (c *BookClient) Get(ctx context.Context, id string) (*pb.BookResponse, error) {
	if err := validateBookID(id); err != nil {
		c.logger.WithError(err).Error("Invalid book ID")
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	c.logger.WithField("book_id", id).Info("Getting book")

	resp, err := c.client.GetBook(ctx, &pb.GetBookRequest{
		BookId: id,
	})

	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"book_id": id,
			"error":   err,
		}).Error("Failed to get book")
		return nil, err
	}

	c.logger.WithField("book_id", id).Info("Book retrieved successfully")
	return resp, nil
}

func (c *BookClient) Update(ctx context.Context, id string) (*pb.BookResponse, error) {
	if err := validateBookID(id); err != nil {
		return nil, err
	}

	c.logger.WithField("book_id", id).Info("Updating book status")

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.UpdateBookStatus(ctx, &pb.UpdateBookRequest{
		BookId: id,
	})

	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"book_id": id,
			"error":   err,
		}).Error("Failed to update book")
		return nil, err
	}

	return resp, nil
}

func (c *BookClient) Close() error {
	c.logger.Info("Closing book client connection")
	return c.conn.Close()
}

func validateBookID(id string) error {
	if id == "" {
		return status.Error(codes.InvalidArgument, "BookId cannot be empty")
	}

	bookID, err := strconv.Atoi(id)
	if err != nil || bookID <= 0 {
		return status.Error(codes.InvalidArgument, "BookId must be a positive integer")
	}
	return nil
}
