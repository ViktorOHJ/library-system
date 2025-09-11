package clients

import (
	"time"

	notificlient "github.com/ViktorOHJ/library-system/notifications/client"
	"github.com/sirupsen/logrus"
)

func GetNotificationsClient(logger *logrus.Logger) (*notificlient.NotificClient, error) {
	notificationsClient, err := notificlient.NewNotificationClient("localhost:50054", 10*time.Second, logger)
	if err != nil {
		logger.Fatalf("NotificationClient not created: %v", err)
		return nil, err
	}
	logger.Info("Notificatios client created successfully")
	return notificationsClient, nil
}
