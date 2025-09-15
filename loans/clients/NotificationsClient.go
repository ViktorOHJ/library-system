package clients

import (
	"time"

	notificlient "github.com/ViktorOHJ/library-system/notifications/client"
	"github.com/sirupsen/logrus"
)

func GetNotificationsClient(addr string, logger *logrus.Logger) (*notificlient.NotificClient, error) {
	if addr == "" {
		logger.Warn("No NOTIFICATIONS_PORT env var set, using default localhost:50052")
		addr = "localhost:50054"
	}
	notificationsClient, err := notificlient.NewNotificationClient(addr, 10*time.Second, logger)
	if err != nil {
		logger.Fatalf("NotificationClient not created: %v", err)
		return nil, err
	}
	logger.Info("Notificatios client created successfully")
	return notificationsClient, nil
}
