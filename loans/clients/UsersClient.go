package clients

import (
	"time"

	"github.com/sirupsen/logrus"

	userclient "github.com/ViktorOHJ/library-system/users/client"
)

func GetUsersClient(addr string, logger *logrus.Logger) (*userclient.UserClient, error) {
	if addr == "" {
		logger.Warn("No USERS_PORT env var set, using default localhost:50052")
		addr = "localhost:50051"
	}

	usercl, err := userclient.NewUserClient(addr, 10*time.Second, logger)
	if err != nil {
		logger.Fatalf("userclient not created: %v", err)
		return nil, err
	}
	logger.Info("User client created successfully")
	return usercl, nil
}
