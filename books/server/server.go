package bookserver

import (
	"context"
	"errors"
	"strconv"
	"time"

	pb "github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BooksServer struct {
	pb.UnimplementedBookServiceServer
	db     *pgxpool.Pool
	logger *logrus.Logger
}

func NewBooksServer(db *pgxpool.Pool, logger *logrus.Logger) *BooksServer {
	return &BooksServer{db: db,
		logger: logger}
}

func (s *BooksServer) CreateBook(parentCtx context.Context, req *pb.CreateBookRequest) (res *pb.BookResponse, err error) {
	s.logger.Info("CreateBook called")

	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	book := &pb.BookResponse{}
	err = s.db.QueryRow(ctx, `INSERT INTO books (title, author, published_year, is_available)
	VALUES ($1, $2, $3, $4) RETURNING id, title, author, published_year, is_available`,
		req.Title, req.Author, req.Year, true).Scan(&book.Id, &book.Title, &book.Author, &book.Year, &book.Available)
	if err != nil {
		s.logger.Errorf("Database error: %v", err)
		return nil, status.Errorf(codes.Internal, "server error")
	}
	return book, nil
}

func (s *BooksServer) GetBook(parentCtx context.Context, req *pb.GetBookRequest) (res *pb.BookResponse, err error) {
	s.logger.Info("GetBook called")
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	if req == nil {
		s.logger.Error("GetBook called with nil request")
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.BookId == "" {
		s.logger.Errorf("GetBook called with empty BookId: %v", err)
		return nil, status.Error(codes.InvalidArgument, "BookId cannot be empty")
	}

	id, err := strconv.Atoi(req.BookId)
	if err != nil || id <= 0 {
		s.logger.Errorf("Invalid BookId format: %v", err)
		return nil, status.Error(codes.InvalidArgument, "Invalid BookId format")
	}

	res = &pb.BookResponse{}
	row := s.db.QueryRow(ctx, "SELECT id, title, author, published_year, is_available FROM books WHERE id=$1", id)
	err = row.Scan(&res.Id, &res.Title, &res.Author, &res.Year, &res.Available)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "book not found")
		}
		s.logger.Errorf("db error: %v", err)
		return nil, status.Error(codes.Internal, "server error")
	}
	return res, nil
}

func (s *BooksServer) UpdateBookStatus(ctx context.Context, req *pb.UpdateBookRequest) (res *pb.BookResponse, err error) {
	s.logger.Info("updateBookStatus called")
	_, err = s.db.Exec(ctx, `UPDATE books SET is_available = NOT is_available WHERE id = $1;`, req.BookId)
	if err != nil {
		s.logger.Errorf("db error: %v", err)
		return nil, status.Error(codes.Internal, "server error")
	}
	return &pb.BookResponse{}, nil
}
