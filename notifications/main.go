package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	notificserver "github.com/ViktorOHJ/library-system/notifications/server"
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
		logger.Fatal("Error loading .env file")
	}
	notificServer := notificserver.NewNotificServer(logger)
	server := grpc.NewServer()
	pb.RegisterNotificationServiceServer(server, notificServer)

	PORT := os.Getenv("NOTIFICATIONS_PORT")
	if PORT == "" {
		PORT = "50054"
		logger.Infof("NOTIFICATIONS_PORT not set, using default port %s", PORT)
	}

	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logger.Fatalf("Failed to listen: %v", err)
	}

	logger.Infof("Listening on port %s", PORT)

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		if err := server.Serve(lis); err != nil {
			logrus.Fatalf("Failed to serve: %v", err)
		}
	}()

	<-shutdownChan
	logger.Info("Received shutdown signal, stopping server gracefully...")
	notificServer.Shutdown()
	server.GracefulStop()
	logger.Info("Server stopped gracefully")
}
