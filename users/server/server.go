package userserver

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

const (
	timeout = 10 * time.Second
)

type UserServer struct {
	pb.UnimplementedUserServiceServer
	db     *pgxpool.Pool
	logger *logrus.Logger
}

func NewUserServer(db *pgxpool.Pool, logger *logrus.Logger) *UserServer {
	return &UserServer{db: db,
		logger: logger}
}

func (s *UserServer) CreateUser(parentCtx context.Context, req *pb.CreateUserRequest) (res *pb.UserResponse, err error) {
	s.logger.Info("CreateUser called")

	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	user := &pb.UserResponse{}
	err = s.db.QueryRow(ctx, "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email",
		req.Name, req.Email).Scan(&user.Id, &user.Name, &user.Email)
	if err != nil {
		s.logger.Errorf("Database error: %v", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}
	return user, nil
}

func (s *UserServer) GetUser(parentCtx context.Context, req *pb.GetUserRequest) (res *pb.UserResponse, err error) {
	s.logger.Info("GetUser called")
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	res = &pb.UserResponse{}

	if req == nil {
		s.logger.Error("GetUser called with nil request")
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.UserId == "" {
		s.logger.Errorf("GetUser called with empty UserId: %v", err)
		return nil, status.Error(codes.InvalidArgument, "UserId cannot be empty")
	}

	id, err := strconv.Atoi(req.UserId)
	if err != nil || id <= 0 {
		s.logger.Errorf("Invalid UserId format: %v", err)
		return nil, status.Error(codes.InvalidArgument, "Invalid UserId format")
	}

	row := s.db.QueryRow(ctx, "SELECT id, name, email FROM users WHERE id=$1", id)

	err = row.Scan(&res.Id, &res.Name, &res.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		s.logger.Errorf("Database error: %v", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return res, nil
}
