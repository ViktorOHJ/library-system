package clients

import (
	"time"

	bookclient "github.com/ViktorOHJ/library-system/books/client"
	"github.com/sirupsen/logrus"
)

func GetBookClient(logger *logrus.Logger) (*bookclient.BookClient, error) {
	bookcl, err := bookclient.NewBookClient("localhost:50052", 10*time.Second, logger)
	if err != nil {
		logger.Fatalf("bookclient not created: %v", err)
		return nil, err
	}
	logger.Info("Book client created successfully")
	return bookcl, nil

}
