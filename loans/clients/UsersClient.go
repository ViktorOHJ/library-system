package clients

import (
	"time"

	"github.com/sirupsen/logrus"

	userclient "github.com/ViktorOHJ/library-system/users/client"
)

func GetUsersClient(logger *logrus.Logger) (*userclient.UserClient, error) {

	usercl, err := userclient.NewUserClient("localhost:50051", 10*time.Second, logger)
	if err != nil {
		logger.Fatalf("userclient not created: %v", err)
		return nil, err
	}
	logger.Info("User client created successfully")
	return usercl, nil
}
