package clients

import (
	"time"

	bookclient "github.com/ViktorOHJ/library-system/books/client"
	"github.com/sirupsen/logrus"
)

func GetBookClient() (*bookclient.BookClient, error) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})
	bookcl, err := bookclient.NewBookClient("localhost:50052", 10*time.Second)
	if err != nil {
		logger.Fatalf("bookclient not created: %v", err)
		return nil, err
	}
	logger.Info("Book client created successfully")
	return bookcl, nil

}
