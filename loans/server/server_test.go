package loansserver

import (
	"context"
	"errors"
	"testing"

	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/ViktorOHJ/library-system/rabbit"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type mockUserService struct {
	user *pb.UserResponse
	err  error
}

func (m *mockUserService) Get(ctx context.Context, id string) (*pb.UserResponse, error) {
	return m.user, m.err
}
func (m *mockUserService) Close() error { return nil }

type mockBookService struct {
	book *pb.BookResponse
	err  error
}

func (m *mockBookService) Get(ctx context.Context, id string) (*pb.BookResponse, error) {
	return m.book, m.err
}
func (m *mockBookService) Update(ctx context.Context, id string) (*pb.BookResponse, error) {
	return m.book, m.err
}
func (m *mockBookService) Close() error { return nil }

type mockNotificationService struct {
	err error
}

func (m *mockNotificationService) Send(ctx context.Context, nType string) (*pb.NotificationResponse, error) {
	return &pb.NotificationResponse{}, m.err
}
func (m *mockNotificationService) Close() error { return nil }

type mockMessagePublisher struct {
	err error
}

func (m *mockMessagePublisher) PublishTask(ctx context.Context, logger *logrus.Logger, msg *rabbit.TaskMessage) error {
	return m.err
}
func (m *mockMessagePublisher) Close() {}

func newTestLoansServer() *LoansServer {
	return NewLoansServerWithDeps(
		&pgxpool.Pool{},
		logrus.New(),
		&mockUserService{user: &pb.UserResponse{Name: "Test", Email: "test@mail.com"}, err: nil},
		&mockBookService{book: &pb.BookResponse{Title: "Book", Author: "Author", Available: true}, err: nil},
		&mockNotificationService{err: nil},
		&mockMessagePublisher{err: nil},
	)
}

func TestValidateBorrowRequest(t *testing.T) {
	s := newTestLoansServer()
	req := &pb.BorrowRequest{UserId: "1", BookId: "2"}
	assert.NoError(t, s.validateBorrowRequest(req))
	assert.Error(t, s.validateBorrowRequest(nil))
	assert.Error(t, s.validateBorrowRequest(&pb.BorrowRequest{UserId: "", BookId: "2"}))
	assert.Error(t, s.validateBorrowRequest(&pb.BorrowRequest{UserId: "abc", BookId: "2"}))
	assert.Error(t, s.validateBorrowRequest(&pb.BorrowRequest{UserId: "1", BookId: ""}))
	assert.Error(t, s.validateBorrowRequest(&pb.BorrowRequest{UserId: "1", BookId: "xyz"}))
}

func TestValidateReturnRequest(t *testing.T) {
	s := newTestLoansServer()

	req := &pb.ReturnRequest{LoanId: "1"}
	assert.NoError(t, s.validateReturnRequest(req))
	assert.Error(t, s.validateReturnRequest(nil))
	assert.Error(t, s.validateReturnRequest(&pb.ReturnRequest{LoanId: ""}))
	assert.Error(t, s.validateReturnRequest(&pb.ReturnRequest{LoanId: "abc"}))
}

func TestBorrowBook_BookNotAvailable(t *testing.T) {
	s := NewLoansServerWithDeps(
		&pgxpool.Pool{},
		logrus.New(),
		&mockUserService{user: &pb.UserResponse{Name: "Test"}, err: nil},
		&mockBookService{book: &pb.BookResponse{Title: "Book", Author: "Author", Available: false}, err: nil},
		&mockNotificationService{err: nil},
		&mockMessagePublisher{err: nil},
	)
	req := &pb.BorrowRequest{UserId: "1", BookId: "2"}
	resp, err := s.BorrowBook(context.Background(), req)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestBorrowBook_UserNotFound(t *testing.T) {
	s := NewLoansServerWithDeps(
		&pgxpool.Pool{},
		logrus.New(),
		&mockUserService{user: nil, err: errors.New("not found")},
		&mockBookService{book: &pb.BookResponse{Available: true}, err: nil},
		&mockNotificationService{err: nil},
		&mockMessagePublisher{err: nil},
	)
	req := &pb.BorrowRequest{UserId: "1", BookId: "2"}
	resp, err := s.BorrowBook(context.Background(), req)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestBorrowBook_BookNotFound(t *testing.T) {
	s := NewLoansServerWithDeps(
		&pgxpool.Pool{},
		logrus.New(),
		&mockUserService{user: &pb.UserResponse{Name: "Test"}, err: nil},
		&mockBookService{book: nil, err: errors.New("not found")},
		&mockNotificationService{err: nil},
		&mockMessagePublisher{err: nil},
	)
	req := &pb.BorrowRequest{UserId: "1", BookId: "2"}
	resp, err := s.BorrowBook(context.Background(), req)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestReturnBook_LoanNotFound(t *testing.T) {
	s := newTestLoansServer()
	s.getLoanInfo = func(ctx context.Context, loanID string) (*LoanInfo, error) {
		return nil, errors.New("not found")
	}
	req := &pb.ReturnRequest{LoanId: "1"}
	resp, err := s.ReturnBook(context.Background(), req)
	assert.Nil(t, resp)
	assert.Error(t, err)
}
