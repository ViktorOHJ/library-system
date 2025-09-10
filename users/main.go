package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/ViktorOHJ/library-system/protos/pb"
	userserver "github.com/ViktorOHJ/library-system/users/server"
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
		logger.Fatal("Error loading .env file")
	}
	dbUrl := os.Getenv("USR_DBURL")
	if dbUrl == "" {
		logger.Fatal("USR_DBURL not set in .env file")
	}

	path := os.Getenv("USERS_MIGRATIONS_PATH")
	if path == "" {
		path = "file://users/migrations"
	}

	runMigrations(path, dbUrl, logger)
	logger.Info("Running migrations...")

	ctx := context.Background()
	db, err := InitDB(ctx, dbUrl, logger)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	logger.Info("Database connection established")
	defer db.Close()

	userServer := userserver.NewUserServer(db, logger)
	server := grpc.NewServer()
	pb.RegisterUserServiceServer(server, userServer)

	PORT := os.Getenv("USERS_PORT")
	if PORT == "" {
		PORT = "50051"
		logger.Infof("USERS_PORT not set, using default port %s", PORT)
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
