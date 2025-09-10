package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	loansserver "github.com/ViktorOHJ/library-system/loans/server"
	pb "github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})

	err := godotenv.Load(".env")
	if err != nil {
		logger.Fatalf("Error loading .env file: %v", err)
	}
	dbUrl := os.Getenv("LOANS_DBURL")
	if dbUrl == "" {
		logger.Fatalf("LOANS_DBURL not set in .env file: %v", err)
	}
	path := os.Getenv("LOANS_MIGRATIONS_PATH")
	if path == "" {
		logger.Info("use default value for loans migrations path")
		path = "file://loans/migrations"
	}
	runMigrations(path, dbUrl, logger)

	logger.Info("Running migrations...")
	ctx := context.Background()
	db, err := InitDB(ctx, dbUrl)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	logger.Info("Database connection established")
	defer db.Close()

	loansServer := loansserver.NewLoansServer(db, logger)
	server := grpc.NewServer()
	pb.RegisterLoanServiceServer(server, loansServer)

	PORT := os.Getenv("LOANS_PORT")
	if PORT == "" {
		PORT = "50053"
		logger.Infof("LOANS_PORT not set, using default port %s", PORT)
	}

	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logger.Fatalf("Failed to listen: %v", err)
	}
	logger.Infof("Listening on port %s", PORT)

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
