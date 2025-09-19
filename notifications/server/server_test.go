package notificserver

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ViktorOHJ/library-system/protos/pb"
	"github.com/ViktorOHJ/library-system/rabbit"
	"github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MockEmailSender struct {
	mock.Mock
}

func (m *MockEmailSender) SendEmail(to, subject, body string) error {
	args := m.Called(to, subject, body)
	return args.Error(0)
}

type MockMessageConsumer struct {
	mock.Mock
	messages chan amqp091.Delivery
}

func (m *MockMessageConsumer) ConsumeFromQueue(queueName string) (<-chan amqp091.Delivery, error) {
	args := m.Called(queueName)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	return m.messages, nil
}

func (m *MockMessageConsumer) Close() {
	m.Called()
	if m.messages != nil {
		close(m.messages)
	}
}

func createTestMessage(msgType, userName, email string) rabbit.TaskMessage {
	return rabbit.TaskMessage{
		Type:       msgType,
		UserName:   userName,
		BookTitle:  "Test Book",
		BookAuthor: "Test Author",
		DueDate:    "2024-12-31",
		LoanID:     "123",
		Email:      email,
	}
}

func createMockDelivery(message rabbit.TaskMessage) amqp091.Delivery {
	messageBytes, _ := json.Marshal(message)
	return amqp091.Delivery{
		Body: messageBytes,
	}
}

func TestNotificServer_SendNotification_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockEmailSender := new(MockEmailSender)
	mockConsumer := new(MockMessageConsumer)

	server := NewNotificServerWithDeps(logger, mockEmailSender, mockConsumer)

	testMessage := createTestMessage("Borrow", "John Doe", "john@example.com")
	mockDelivery := createMockDelivery(testMessage)

	messages := make(chan amqp091.Delivery, 1)
	messages <- mockDelivery
	close(messages)

	mockConsumer.messages = messages
	mockConsumer.On("ConsumeFromQueue", "borrow_queue").Return(messages, nil)

	mockEmailSender.On("SendEmail",
		"john@example.com",
		"Book Borrowed Notification",
		mock.MatchedBy(func(body string) bool {
			return assert.Contains(t, body, "John Doe") &&
				assert.Contains(t, body, "Test Book")
		})).Return(nil)

	req := &pb.NotificationRequest{
		NotificationType: "borrow_queue",
	}

	resp, err := server.SendNotification(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, resp.Success)
	mockEmailSender.AssertExpectations(t)
	mockConsumer.AssertExpectations(t)
}

func TestNotificServer_SendNotification_EmailFailure(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockEmailSender := new(MockEmailSender)
	mockConsumer := new(MockMessageConsumer)

	server := NewNotificServerWithDeps(logger, mockEmailSender, mockConsumer)

	testMessage := createTestMessage("Borrow", "Jane Doe", "jane@example.com")
	mockDelivery := createMockDelivery(testMessage)

	messages := make(chan amqp091.Delivery, 1)
	messages <- mockDelivery
	close(messages)

	mockConsumer.messages = messages
	mockConsumer.On("ConsumeFromQueue", "borrow_queue").Return(messages, nil)
	mockEmailSender.On("SendEmail", mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("SMTP connection failed"))

	req := &pb.NotificationRequest{
		NotificationType: "borrow_queue",
	}

	resp, err := server.SendNotification(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, resp.Success)
	mockEmailSender.AssertExpectations(t)
}

func TestNotificServer_SendNotification_ConsumerFailure(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockEmailSender := new(MockEmailSender)
	mockConsumer := new(MockMessageConsumer)

	server := NewNotificServerWithDeps(logger, mockEmailSender, mockConsumer)

	mockConsumer.On("ConsumeFromQueue", "invalid_queue").
		Return(nil, fmt.Errorf("queue not found"))

	req := &pb.NotificationRequest{
		NotificationType: "invalid_queue",
	}

	resp, err := server.SendNotification(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())

	mockConsumer.AssertExpectations(t)
}

func TestNotificServer_ProcessMessage_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockEmailSender := new(MockEmailSender)
	server := NewNotificServerWithDeps(logger, mockEmailSender, nil)

	testMessage := createTestMessage("Return", "Bob Smith", "bob@example.com")
	messageBytes, _ := json.Marshal(testMessage)

	mockEmailSender.On("SendEmail",
		"bob@example.com",
		"Book Return Confirmation",
		mock.MatchedBy(func(body string) bool {
			return assert.Contains(t, body, "Bob Smith") &&
				assert.Contains(t, body, "Test Book")
		})).Return(nil)

	err := server.processMessage(messageBytes)
	require.NoError(t, err)
	mockEmailSender.AssertExpectations(t)
}

func TestNotificServer_ProcessMessage_InvalidJSON(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	server := NewNotificServerWithDeps(logger, nil, nil)

	invalidJSON := []byte(`{"invalid": json}`)

	err := server.processMessage(invalidJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal message")
}

func TestNotificServer_ProcessMessage_EmptyEmail(t *testing.T) {
	// Arrange
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	server := NewNotificServerWithDeps(logger, nil, nil)

	testMessage := createTestMessage("Borrow", "Test User", "")
	messageBytes, _ := json.Marshal(testMessage)

	err := server.processMessage(messageBytes)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "email is empty")
}

func TestNotificServer_EdgeCases(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	testCases := []struct {
		name        string
		messageJSON string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid message",
			messageJSON: `{"type":"Borrow","user_name":"Test","email":"test@example.com","book_title":"Book","book_author":"Author"}`,
			expectError: false,
		},
		{
			name:        "missing email field",
			messageJSON: `{"type":"Borrow","user_name":"Test","book_title":"Book"}`,
			expectError: true,
			errorMsg:    "email is empty",
		},
		{
			name:        "empty email",
			messageJSON: `{"type":"Borrow","user_name":"Test","email":"","book_title":"Book"}`,
			expectError: true,
			errorMsg:    "email is empty",
		},
		{
			name:        "malformed JSON",
			messageJSON: `{"type":"Borrow","user_name":}`,
			expectError: true,
			errorMsg:    "failed to unmarshal message",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockEmailSender := new(MockEmailSender)
			if !tc.expectError {
				mockEmailSender.On("SendEmail", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			}

			server := NewNotificServerWithDeps(logger, mockEmailSender, nil)

			err := server.processMessage([]byte(tc.messageJSON))

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
