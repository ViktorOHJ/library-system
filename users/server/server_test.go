package userserver

import (
	"context"
	"os"
	"testing"

	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/status"
)

func setupTestDB(t *testing.T, logger *logrus.Logger) *pgxpool.Pool {
	err := godotenv.Load(".env")
	if err != nil {
		logger.Fatalf("Error loading .env file: %v", err)
	}
	dbURL := os.Getenv("TEST_DBURL")

	pool, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)

	_, err = pool.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL
)`)
	require.NoError(t, err)

	return pool
}

func TestUserServer_CreateUser_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	db := setupTestDB(t, logger)
	defer db.Close()
	server := NewUserServer(db, logger)

	req := &pb.CreateUserRequest{
		Name:  "Test User",
		Email: "test@gmail.com",
	}
	resp, err := server.CreateUser(context.Background(), req)

	require.NoError(t, err)
	require.NotEmpty(t, resp.Id)
	require.Equal(t, req.Name, resp.Name)
	require.Equal(t, req.Email, resp.Email)
}

func TestUserServer_CreateUser_InvalidArgument(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	db := setupTestDB(t, logger)
	defer db.Close()
	server := NewUserServer(db, logger)

	tests := []struct {
		name    string
		req     *pb.CreateUserRequest
		errCode string
	}{
		{
			name: "Empty Name",
			req: &pb.CreateUserRequest{
				Name:  "",
				Email: "test@gmail.com",
			},
			errCode: "InvalidArgument",
		},
		{
			name: "Empty Email",
			req: &pb.CreateUserRequest{
				Name:  "Test User",
				Email: "",
			},
			errCode: "InvalidArgument",
		},
		{
			name: "Invalid Email Format",
			req: &pb.CreateUserRequest{
				Name:  "Test User",
				Email: "invalid-email",
			},
			errCode: "InvalidArgument",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.CreateUser(context.Background(), tt.req)
			require.Nil(t, resp)
			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, tt.errCode, st.Code().String())
		})
	}
}

func TestUserServer_GetUser_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	db := setupTestDB(t, logger)
	defer db.Close()
	server := NewUserServer(db, logger)

	req := &pb.GetUserRequest{
		UserId: "1",
	}
	resp, err := server.GetUser(context.Background(), req)

	require.NoError(t, err)
	require.NotEmpty(t, resp.Id)
	require.Equal(t, "1", resp.Id)
}

func TestUserServer_GetUser_InvalidArgument(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	db := setupTestDB(t, logger)
	defer db.Close()
	server := NewUserServer(db, logger)

	tests := []struct {
		name    string
		req     *pb.GetUserRequest
		errCode string
	}{
		{
			name: "Empty UserId",
			req: &pb.GetUserRequest{
				UserId: "",
			},
			errCode: "InvalidArgument",
		},
		{
			name: "Non-numeric UserId",
			req: &pb.GetUserRequest{
				UserId: "abc",
			},
			errCode: "InvalidArgument",
		},
		{
			name:    "Nil Request",
			req:     nil,
			errCode: "InvalidArgument",
		},
		{
			name: "Negative UserId",
			req: &pb.GetUserRequest{
				UserId: "-1",
			},
			errCode: "InvalidArgument",
		},
		{
			name: "Zero UserId",
			req: &pb.GetUserRequest{
				UserId: "0",
			},
			errCode: "InvalidArgument",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.GetUser(context.Background(), tt.req)
			require.Nil(t, resp)
			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, tt.errCode, st.Code().String())
		})
	}
}
