package clients

import (
	"time"

	bookclient "github.com/ViktorOHJ/library-system/books/client"
	"github.com/sirupsen/logrus"
)

func GetBookClient(addr string, logger *logrus.Logger) (*bookclient.BookClient, error) {
	if addr == "" {
		logger.Warn("No BOOKS_PORT env var set, using default localhost:50052")
		addr = "localhost:50052"
	}
	bookcl, err := bookclient.NewBookClient(addr, 10*time.Second, logger)
	if err != nil {
		logger.Fatalf("bookclient not created: %v", err)
		return nil, err
	}
	logger.Info("Book client created successfully")
	return bookcl, nil

}
