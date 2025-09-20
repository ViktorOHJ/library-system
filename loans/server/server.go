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
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserService interface {
	Get(ctx context.Context, id string) (*pb.UserResponse, error)
	Close() error
}

type BookService interface {
	Get(ctx context.Context, id string) (*pb.BookResponse, error)
	Update(ctx context.Context, id string) (*pb.BookResponse, error)
	Close() error
}

type NotificationService interface {
	Send(ctx context.Context, nType string) (*pb.NotificationResponse, error)
	Close() error
}

type MessagePublisher interface {
	PublishTask(ctx context.Context, logger *logrus.Logger, message *rabbit.TaskMessage) error
	Close()
}

type LoansServer struct {
	pb.UnimplementedLoanServiceServer
	db                  *pgxpool.Pool
	logger              *logrus.Logger
	userService         UserService
	bookService         BookService
	notificationService NotificationService
	messagePublisher    MessagePublisher
	getLoanInfo         func(ctx context.Context, loanID string) (*LoanInfo, error)
}

func NewLoansServer(db *pgxpool.Pool, logger *logrus.Logger) *LoansServer {
	return &LoansServer{
		db:     db,
		logger: logger,
	}
}

func NewLoansServerWithDeps(
	db *pgxpool.Pool,
	logger *logrus.Logger,
	userService UserService,
	bookService BookService,
	notificationService NotificationService,
	messagePublisher MessagePublisher,
) *LoansServer {
	return &LoansServer{
		db:                  db,
		logger:              logger,
		userService:         userService,
		bookService:         bookService,
		notificationService: notificationService,
		messagePublisher:    messagePublisher,
	}
}

func (s *LoansServer) BorrowBook(parentCtx context.Context, req *pb.BorrowRequest) (*pb.LoanResponse, error) {
	s.logger.Info("BorrowBook called")

	if err := s.validateBorrowRequest(req); err != nil {
		return nil, err
	}
	if err := s.initServices(); err != nil {
		return nil, status.Error(codes.Internal, "server error")
	}

	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	user, err := s.userService.Get(ctx, req.UserId)
	if err != nil {
		s.logger.Errorf("Failed to get user %s: %v", req.UserId, err)
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	book, err := s.bookService.Get(ctx, req.BookId)
	if err != nil {
		s.logger.Errorf("Failed to get book %s: %v", req.BookId, err)
		return nil, status.Errorf(codes.NotFound, "book not found")
	}

	if !book.Available {
		return nil, status.Error(codes.InvalidArgument, "book is not available")
	}

	loanID, err := s.createLoanRecord(ctx, req.UserId, req.BookId)
	if err != nil {
		s.logger.Errorf("Failed to create loan record: %v", err)
		return nil, status.Error(codes.Internal, "failed to create loan")
	}

	if err := s.updateBookAvailability(ctx, req.BookId); err != nil {
		s.logger.Errorf("Failed to update book availability: %v", err)
		return nil, status.Error(codes.Internal, "failed to update book status")
	}

	if err := s.publishBorrowMessage(ctx, user, book, loanID); err != nil {
		s.logger.Errorf("Failed to publish message: %v", err)
	}

	go s.sendNotificationAsync(user, book, "borrow_queue")

	return &pb.LoanResponse{
		Id:           strconv.Itoa(loanID),
		User:         user,
		Book:         book,
		BorrowedDate: time.Now().Format(time.RFC3339),
		DueDate:      time.Now().AddDate(0, 0, 14).Format("2006-01-02"),
	}, nil
}

func (s *LoansServer) ReturnBook(parentCtx context.Context, req *pb.ReturnRequest) (*pb.LoanResponse, error) {
	s.logger.Info("ReturnBook called")

	if err := s.validateReturnRequest(req); err != nil {
		return nil, err
	}
	if err := s.initServices(); err != nil {
		return nil, status.Error(codes.Internal, "server error")
	}

	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	loanInfo, err := s.getLoanInfo(ctx, req.LoanId)
	if err != nil {
		s.logger.Errorf("Failed to get loan info: %v", err)
		return nil, status.Error(codes.NotFound, "loan not found")
	}

	user, err := s.userService.Get(ctx, loanInfo.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	book, err := s.bookService.Get(ctx, loanInfo.BookID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get book")
	}

	if err := s.returnBookTransaction(ctx, req.LoanId, loanInfo.BookID); err != nil {
		s.logger.Errorf("Failed to return book: %v", err)
		return nil, status.Error(codes.Internal, "failed to return book")
	}

	if err := s.publishReturnMessage(ctx, user, book, req.LoanId); err != nil {
		s.logger.Errorf("Failed to publish return message: %v", err)
	}

	go s.sendNotificationAsync(user, book, "return_queue")

	return &pb.LoanResponse{
		Id:           req.LoanId,
		User:         user,
		Book:         book,
		ReturnedDate: time.Now().Format("2006-01-02"),
	}, nil
}

func (s *LoansServer) validateBorrowRequest(req *pb.BorrowRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.UserId == "" {
		return status.Error(codes.InvalidArgument, "user id cannot be empty")
	}
	if req.BookId == "" {
		return status.Error(codes.InvalidArgument, "book id cannot be empty")
	}

	if _, err := strconv.Atoi(req.UserId); err != nil {
		return status.Error(codes.InvalidArgument, "invalid user id format")
	}
	if _, err := strconv.Atoi(req.BookId); err != nil {
		return status.Error(codes.InvalidArgument, "invalid book id format")
	}

	return nil
}

func (s *LoansServer) validateReturnRequest(req *pb.ReturnRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.LoanId == "" {
		return status.Error(codes.InvalidArgument, "loan id cannot be empty")
	}
	if _, err := strconv.Atoi(req.LoanId); err != nil {
		return status.Error(codes.InvalidArgument, "invalid loan id format")
	}
	return nil
}

func (s *LoansServer) createLoanRecord(ctx context.Context, userID, bookID string) (int, error) {
	var loanID int
	err := s.db.QueryRow(ctx,
		`INSERT INTO loans (user_id, book_id, loan_date, return_date)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, bookID, time.Now(), time.Now().AddDate(0, 0, 14)).Scan(&loanID)

	if err != nil {
		return 0, err
	}

	s.logger.WithFields(logrus.Fields{
		"loan_id": loanID,
		"user_id": userID,
		"book_id": bookID,
	}).Info("Loan record created")

	return loanID, nil
}

func (s *LoansServer) updateBookAvailability(ctx context.Context, bookID string) error {
	_, err := s.bookService.Update(ctx, bookID)
	return err
}

type LoanInfo struct {
	UserID string
	BookID string
}

func (s *LoansServer) GetLoanInfo(ctx context.Context, loanID string) (*LoanInfo, error) {
	var userID, bookID string
	row := s.db.QueryRow(ctx, "SELECT user_id, book_id FROM loans WHERE id = $1", loanID)
	err := row.Scan(&userID, &bookID)
	if err != nil {
		return nil, err
	}

	return &LoanInfo{
		UserID: userID,
		BookID: bookID,
	}, nil
}

func (s *LoansServer) returnBookTransaction(ctx context.Context, loanID, bookID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "DELETE FROM loans WHERE id = $1", loanID)
	if err != nil {
		return err
	}

	_, err = s.bookService.Update(ctx, bookID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *LoansServer) publishBorrowMessage(ctx context.Context, user *pb.UserResponse, book *pb.BookResponse, loanID int) error {
	if s.messagePublisher == nil {
		return errors.New("message publisher not initialized")
	}

	message := &rabbit.TaskMessage{
		Type:       "Borrow",
		UserName:   user.Name,
		BookTitle:  book.Title,
		BookAuthor: book.Author,
		DueDate:    time.Now().AddDate(0, 0, 14).Format("2006-01-02"),
		LoanID:     strconv.Itoa(loanID),
		Email:      user.Email,
	}

	return s.messagePublisher.PublishTask(ctx, s.logger, message)
}

func (s *LoansServer) publishReturnMessage(ctx context.Context, user *pb.UserResponse, book *pb.BookResponse, loanID string) error {
	if s.messagePublisher == nil {
		return errors.New("message publisher not initialized")
	}

	message := &rabbit.TaskMessage{
		Type:       "Return",
		UserName:   user.Name,
		BookTitle:  book.Title,
		BookAuthor: book.Author,
		DueDate:    time.Now().Format("2006-01-02"),
		LoanID:     loanID,
		Email:      user.Email,
	}

	return s.messagePublisher.PublishTask(ctx, s.logger, message)
}

func (s *LoansServer) sendNotificationAsync(user *pb.UserResponse, book *pb.BookResponse, queueType string) {
	if s.notificationService == nil {
		s.logger.Warn("Notification service not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := s.notificationService.Send(ctx, queueType)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.logger.Errorf("Failed to send notification: %v", err)
	} else {
		s.logger.WithFields(logrus.Fields{
			"user":       user.Name,
			"book":       book.Title,
			"queue_type": queueType,
		}).Info("Notification sent successfully")
	}
}

func (s *LoansServer) initServices() (err error) {
	err = godotenv.Load(".env")
	if err != nil {
		s.logger.Warn("No .env file found, relying on environment variables")
		return err
	}
	s.userService, err = clients.GetUsersClient(os.Getenv("USERS_PORT"), s.logger)
	if err != nil {
		s.logger.Errorf("Failed to initialize user service: %v", err)
		return err
	}
	s.bookService, err = clients.GetBookClient(os.Getenv("BOOKS_PORT"), s.logger)
	if err != nil {
		s.logger.Errorf("Failed to initialize book service: %v", err)
		return err
	}
	s.notificationService, err = clients.GetNotificationsClient(os.Getenv("NOTIFICATIONS_PORT"), s.logger)
	if err != nil {
		s.logger.Errorf("Failed to initialize notification service: %v", err)
		return err
	}

	s.messagePublisher, err = rabbit.NewRabbitMQClient(s.logger, os.Getenv("RABBIT_URL"))
	if err != nil {
		s.logger.Errorf("Failed to initialize RabbitMQ client: %v", err)
		return err
	}
	return nil
}

func (s *LoansServer) Shutdown() {
	if s.userService != nil {
		s.userService.Close()
	}
	if s.bookService != nil {
		s.bookService.Close()
	}
	if s.notificationService != nil {
		s.notificationService.Close()
	}
	if s.messagePublisher != nil {
		s.messagePublisher.Close()
	}
	if s.db != nil {
		s.db.Close()
	}
}
