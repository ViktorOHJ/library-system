package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	bookserver "github.com/ViktorOHJ/library-system/books/server"
	pb "github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func runMigrations(db, path string) {
	m, err := migrate.New(
		path,
		db,
	)
	if err != nil {
		logrus.Fatal(err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logrus.Fatal("Migrations failed: ", err)
	}
}

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})

	err := godotenv.Load(".env")
	if err != nil {
		logger.Fatal("Error loading .env file")
	}
	dbUrl := os.Getenv("BOOKS_DBURL")
	if dbUrl == "" {
		logger.Fatal("BOOKS_DBURL not set in .env file")
	}

	path := os.Getenv("BOOKS_MIGRATIONS_PATH")
	if path == "" {
		path = "file://books/migrations"
	}
	runMigrations(dbUrl, path)
	logger.Info("Running migrations...")

	ctx := context.Background()
	db, err := InitDB(ctx, dbUrl)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	logger.Info("Database connection established")
	defer db.Close()

	booksServer := bookserver.NewBooksServer(db, logger)
	server := grpc.NewServer()
	pb.RegisterBookServiceServer(server, booksServer)

	PORT := os.Getenv("BOOKS_PORT")
	if PORT == "" {
		PORT = "50052" // Default port if not set
		logger.Infof("BOOKS_PORT not set, using default port %s", PORT)
	}

	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logger.Fatalf("Failed to listen: %v", err)
	}

	logger.Infof("Listening on port %s", PORT)

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		if err := server.Serve(lis); err != nil {
			logrus.Fatalf("Failed to serve: %v", err)
		}
	}()

	<-stop
	logger.Info("Received shutdown signal, stopping server gracefully...")
	server.GracefulStop()
	logger.Info("Server stopped gracefully")
}
