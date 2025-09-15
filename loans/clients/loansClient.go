package clients

import (
	"context"
	"time"

	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type LoansClient struct {
	client  pb.LoanServiceClient
	timeout time.Duration
	conn    *grpc.ClientConn
	logger  *logrus.Logger
}

func NewLoansClient(addr string, timeout time.Duration, logger *logrus.Logger) (*LoansClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &LoansClient{
		conn:    conn,
		timeout: timeout,
		client:  pb.NewLoanServiceClient(conn),
		logger:  logger,
	}, nil
}

func (c *LoansClient) Borrow(ctx context.Context, uID, bID string) (*pb.LoanResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.BorrowBook(ctx, &pb.BorrowRequest{
		UserId: uID,
		BookId: bID,
	})
}

func (c *LoansClient) Return(ctx context.Context, id string) (*pb.LoanResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.ReturnBook(ctx, &pb.ReturnRequest{
		LoanId: id,
	})
}

func (c *LoansClient) Close() error {
	return c.conn.Close()
}
