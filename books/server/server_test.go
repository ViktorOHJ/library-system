package bookserver

import (
	"context"
	"os"
	"testing"

	"time"

	pb "github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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
		CREATE TABLE IF NOT EXISTS books (
			id SERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			author VARCHAR(100) NOT NULL,
			published_year INT NOT NULL,
			is_available BOOLEAN NOT NULL
		)
	`)
	require.NoError(t, err)

	return pool
}

func TestBooksServer_CreateBook_Success(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	db := setupTestDB(t, logger)
	defer db.Close()
	server := NewBooksServer(db, logger)

	req := &pb.CreateBookRequest{
		Title:  "Test Book",
		Author: "Test Author",
		Year:   2023,
	}

	resp, err := server.CreateBook(context.Background(), req)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Id)
	assert.Equal(t, "Test Book", resp.Title)
	assert.Equal(t, "Test Author", resp.Author)
	assert.Equal(t, int32(2023), resp.Year)
	assert.True(t, resp.Available)
}

func TestBooksServer_CreateBook_InvalidInput(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	db := setupTestDB(t, logger)
	defer db.Close()
	server := NewBooksServer(db, logger)
	tests := []struct {
		name    string
		req     *pb.CreateBookRequest
		errCode string
	}{
		{
			name: "Empty Title",
			req: &pb.CreateBookRequest{
				Title:  "",
				Author: "Author",
				Year:   2023,
			},
			errCode: "InvalidArgument",
		},
		{
			name: "Title Too Long",
			req: &pb.CreateBookRequest{
				Title:  string(make([]byte, 256)),
				Author: "Author",
				Year:   2023,
			},
			errCode: "InvalidArgument",
		},
		{
			name: "Empty Author",
			req: &pb.CreateBookRequest{
				Title:  "Title",
				Author: "",
				Year:   2023,
			},
			errCode: "InvalidArgument",
		},
		{
			name: "Author Too Long",
			req: &pb.CreateBookRequest{
				Title:  "Title",
				Author: string(make([]byte, 101)),
				Year:   2023,
			},
			errCode: "InvalidArgument",
		},
		{
			name: "Invalid Year (Negative)",
			req: &pb.CreateBookRequest{
				Title:  "Title",
				Author: "Author",
				Year:   -1,
			},
			errCode: "InvalidArgument",
		},
		{
			name: "Invalid Year (Too Future)",
			req: &pb.CreateBookRequest{
				Title:  "Title",
				Author: "Author",
				Year:   int32(time.Now().Year() + 11),
			},
			errCode: "InvalidArgument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.CreateBook(context.Background(), tt.req)
			require.Nil(t, resp)
			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.errCode, st.Code().String())
		})
	}
}

func TestGetBook_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	db := setupTestDB(t, logger)
	defer db.Close()
	server := NewBooksServer(db, logger)

	req := &pb.GetBookRequest{
		BookId: "1",
	}
	resp, err := server.GetBook(context.Background(), req)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Id)
	assert.Equal(t, "Test Book", resp.Title)
	assert.Equal(t, "Test Author", resp.Author)
	assert.Equal(t, int32(2023), resp.Year)
	assert.True(t, resp.Available)
}

func TestGetBook_InvalidInput(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	db := setupTestDB(t, logger)
	defer db.Close()
	server := NewBooksServer(db, logger)
	tests := []struct {
		name    string
		req     *pb.GetBookRequest
		errCode string
	}{
		{
			name: "empty ID",
			req: &pb.GetBookRequest{
				BookId: "",
			},
			errCode: "InvalidArgument",
		},
		{
			name: "non-numeric ID",
			req: &pb.GetBookRequest{
				BookId: "abc",
			},
			errCode: "InvalidArgument",
		},
		{
			name: "negative ID",
			req: &pb.GetBookRequest{
				BookId: "-1",
			},
			errCode: "InvalidArgument",
		},
		{
			name: "zero ID",
			req: &pb.GetBookRequest{
				BookId: "0",
			},
			errCode: "InvalidArgument",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.GetBook(context.Background(), tt.req)
			require.Nil(t, resp)
			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.errCode, st.Code().String())
		})
	}
}
