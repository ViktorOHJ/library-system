package loansserver

import (
	"context"
	"os"
	"time"

	"github.com/ViktorOHJ/library-system/loans/clients"
	"github.com/ViktorOHJ/library-system/loans/rabbit"
	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoansServer struct {
	pb.UnimplementedLoanServiceServer
	db     *pgxpool.Pool
	logger *logrus.Logger
}

func NewLoansServer(db *pgxpool.Pool, logger *logrus.Logger) *LoansServer {
	return &LoansServer{db: db,
		logger: logger}
}

func (s *LoansServer) BorrowBook(parentCtx context.Context, req *pb.BorrowRequest) (res *pb.LoanResponse, err error) {
	usrClient, err := clients.GetUsersClient(s.logger)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user client: %v", err)
	}
	defer usrClient.Close()

	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	user, err := usrClient.Get(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	bookClient, err := clients.GetBookClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create book client: %v", err)
	}
	defer bookClient.Close()

	book, err := bookClient.Get(ctx, req.BookId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get book: %v", err)
	}
	if !book.Available {
		return nil, status.Error(codes.InvalidArgument, "book is not available")
	}
	id, err := s.db.Exec(ctx, `INSERT INTO loans (user_id, book_id, loan_date, return_date) VALUES ($1, $2, $3, $4) RETURNING id`,
		req.UserId, req.BookId, time.Now(), time.Now().AddDate(0, 0, 14))
	if err != nil {
		s.logger.Errorf("Database error: %v", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}
	message := rabbit.TaskMessage{
		Type:   "Borrow",
		UserID: req.UserId,
		BookID: req.BookId,
		Email:  user.Email,
	}
	err = publish(ctx, s.logger, &message)
	if err != nil {
		s.logger.Errorf("RabbirMQ error: %v", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &pb.LoanResponse{
		Id:           id.String(),
		User:         user,
		Book:         book,
		BorrowedDate: time.Now().Format(time.RFC3339),
		DueDate:      time.Now().AddDate(0, 0, 14).Format(time.RFC3339),
		ReturnedDate: time.Now().AddDate(0, 0, 14).Format(time.RFC3339),
	}, nil
}

func (s *LoansServer) ReturnBook(parentCtx context.Context, req *pb.ReturnRequest) (res *pb.LoanResponse, err error) {
	bookClient, err := clients.GetBookClient()
	if err != nil {
		s.logger.Errorf("ERROR returnbook creating bookclient: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create book client: %v", err)
	}
	s.logger.Info("bookclient create")
	defer bookClient.Close()

	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	userclient, err := clients.GetUsersClient(s.logger)
	if err != nil {
		s.logger.Errorf("error get client: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create user client: %v", err)
	}
	defer userclient.Close()

	var userID, bookID string
	row := s.db.QueryRow(ctx, `SELECT user_id, book_id FROM loans WHERE id = $1`, req.LoanId)
	err = row.Scan(&userID, &bookID)
	if err != nil {
		s.logger.Errorf("err to scan row: %v", err)
		return nil, status.Error(codes.Internal, "Server Error")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.Errorf("Error begin tx: %v", err)
		return nil, status.Error(codes.Internal, "Server Error")
	}
	_, err = tx.Exec(ctx, `DELETE FROM loans WHERE id = $1`, req.LoanId)
	if err != nil {
		s.logger.Errorf("Error deleting loan: %v", err)
		tx.Rollback(ctx)
		return nil, status.Error(codes.Internal, "Server Error")
	}

	book, err := bookClient.Get(ctx, bookID)
	if err != nil {
		s.logger.Errorf("Error of get book: %v", err)
		tx.Rollback(ctx)
		return nil, status.Error(codes.Internal, "Server Error")
	}
	s.logger.Info("getbook ok")
	if book.Available {
		if err != nil {
			s.logger.Errorf("1 Error commit tx: %v", err)
			tx.Rollback(ctx)
			return nil, status.Error(codes.Internal, "Server Error")
		} else {
			s.logger.Errorf("Book is not available for return: %v", bookID)
			tx.Rollback(ctx)
			return nil, status.Error(codes.InvalidArgument, "book is not available for return")
		}
	}
	s.logger.Info("update ok")
	err = tx.Commit(ctx)
	if err != nil {
		s.logger.Errorf("commit tx: %v", err)
		return nil, status.Error(codes.Internal, "Server Error")
	}
	user, err := userclient.Get(ctx, userID)
	message := rabbit.TaskMessage{
		Type:   "Return",
		UserID: userID,
		BookID: bookID,
		Email:  user.Email,
	}
	err = publish(ctx, s.logger, &message)
	if err != nil {
		s.logger.Errorf("error publish: %v", err)
		return nil, status.Error(codes.Internal, "Server Error")
	}
	s.logger.Info("publish ok")
	return res, nil
}

func publish(ctx context.Context, logger *logrus.Logger, message *rabbit.TaskMessage) error {

	rabbitURL := os.Getenv("RABBIT_URL")
	rabbitCl, err := rabbit.NewRabbitMQClient(logger, rabbitURL)
	if err != nil {
		logger.Errorf("Rabbit create client: %v", err)
		return nil
	}
	defer rabbitCl.Close()
	logger.Info("rabbit cliet success")

	err = rabbitCl.PublishTask(ctx, logger, message)
	if err != nil {
		logger.Errorf("Rabbit publish: %v", err)
		return err
	}
	return nil
}
