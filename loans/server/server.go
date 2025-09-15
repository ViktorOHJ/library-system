package loansserver

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/ViktorOHJ/library-system/loans/clients"
	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/ViktorOHJ/library-system/rabbit"
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
	usrClient, err := clients.GetUsersClient(os.Getenv("USERS_PORT"), s.logger)
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

	bookClient, err := clients.GetBookClient(os.Getenv("BOOKS_PORT"), s.logger)
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
	var loanID int
	err = s.db.QueryRow(ctx,
		`INSERT INTO loans (user_id, book_id, loan_date, return_date)
         VALUES ($1, $2, $3, $4) RETURNING id`,
		req.UserId, req.BookId, time.Now(), time.Now().AddDate(0, 0, 14)).Scan(&loanID)
	if err != nil {
		s.logger.Errorf("Database error: %v", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}
	_, err = bookClient.Update(ctx, req.BookId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "server error")
	}
	message := rabbit.TaskMessage{
		Type:       "Borrow",
		UserName:   user.Name,
		BookTitle:  book.Title,
		BookAuthor: book.Author,
		DueDate:    time.Now().AddDate(0, 0, 14).Format("2006-01-02"),
		LoanID:     strconv.Itoa(loanID),
		Email:      user.Email,
	}
	err = publish(ctx, s.logger, &message)
	if err != nil {
		s.logger.Errorf("RabbirMQ error: %v", err)
		return nil, status.Error(codes.Internal, "server error")
	}
	notiCL, err := clients.GetNotificationsClient(os.Getenv("NOTIFICATIONS_PORT"), s.logger)
	if err != nil {
		s.logger.Errorf("Error create noti cliet:%v", err)
		return nil, status.Error(codes.Internal, "server error")
	}
	go func() {
		notifCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := notiCL.Send(notifCtx, req.UserId, "", "borrow_queue")
		if err != nil {
			if isContextCancellationError(err) {
				s.logger.Infof("Notification request was cancelled (this is expected): %v", err)
				return
			}
		}
		s.logger.Info("send succes")
	}()
	return &pb.LoanResponse{
		Id:           strconv.Itoa(loanID),
		User:         user,
		Book:         book,
		BorrowedDate: time.Now().Format(time.RFC3339),
		DueDate:      time.Now().AddDate(0, 0, 14).Format(time.RFC3339),
	}, nil
}

func (s *LoansServer) ReturnBook(parentCtx context.Context, req *pb.ReturnRequest) (res *pb.LoanResponse, err error) {
	bookClient, err := clients.GetBookClient(os.Getenv("BOOKS_PORT"), s.logger)
	if err != nil {
		s.logger.Errorf("ERROR returnbook creating bookclient: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create book client: %v", err)
	}
	s.logger.Info("bookclient create")
	defer bookClient.Close()

	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	userclient, err := clients.GetUsersClient(os.Getenv("USERS_PORT"), s.logger)
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

	statusReq := &pb.UpdateBookRequest{
		BookId: bookID,
	}
	_, err = bookClient.Update(ctx, statusReq.BookId)
	if err != nil {
		s.logger.Errorf("Error updating book status: %v", err)
		tx.Rollback(ctx)
		return nil, status.Error(codes.Internal, "Server Error")
	}
	err = tx.Commit(ctx)
	if err != nil {
		s.logger.Errorf("commit tx: %v", err)
		return nil, status.Error(codes.Internal, "Server Error")
	}
	user, err := userclient.Get(ctx, userID)
	if err != nil {
		s.logger.Errorf("Error getting user: %v", err)
		return nil, status.Error(codes.Internal, "Server Error")
	}
	book, err := bookClient.Get(ctx, bookID)
	if err != nil {
		s.logger.Errorf("Error getting book: %v", err)
		return nil, status.Error(codes.Internal, "Server Error")
	}
	message := rabbit.TaskMessage{
		Type:       "Return",
		UserName:   user.Name,
		BookTitle:  book.Title,
		BookAuthor: book.Author,
		DueDate:    time.Now().Format("2006-01-02"),
		LoanID:     req.LoanId,
		Email:      user.Email,
	}
	err = publish(ctx, s.logger, &message)
	if err != nil {
		s.logger.Errorf("error publish: %v", err)
		return nil, status.Error(codes.Internal, "Server Error")
	}

	notiCL, err := clients.GetNotificationsClient(os.Getenv("NOTIFICATIONS_PORT"), s.logger)
	if err != nil {
		s.logger.Errorf("Error create noti cliet:%v", err)

	}
	go func() {
		notifCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = notiCL.Send(notifCtx, userID, "", "return_queue")
		if err != nil {
			if isContextCancellationError(err) {
				s.logger.Infof("Notification request was cancelled (this is expected): %v", err)
				return
			}
		}
		s.logger.Info("send succes")
	}()
	res = &pb.LoanResponse{
		Id:           req.LoanId,
		User:         user,
		Book:         book,
		ReturnedDate: time.Now().Format("2006-01-02"),
	}
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

func isContextCancellationError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Canceled, codes.DeadlineExceeded:
			return true
		}
	}

	return false
}
