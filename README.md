# Система управления библиотекой

Распределенная система управления библиотекой на основе микросервисов, построенная с использованием Go, gRPC, PostgreSQL и RabbitMQ. Система обрабатывает заимствование и возврат книг, управление пользователями и автоматические email-уведомления.

## Архитектура

Система состоит из четырех основных микросервисов:

- **Сервис пользователей** (Порт 50051) - Управление пользователями и аутентификация
- **Сервис книг** (Порт 50052) - Управление инвентарем книг
- **Сервис займов** (Порт 50053) - Логика заимствования и возврата книг
- **Сервис уведомлений** (Порт 50054) - Email-уведомления через RabbitMQ

## Функциональность

- ✅ Регистрация и управление пользователями
- ✅ Управление инвентарем книг
- ✅ Заимствование и возврат книг
- ✅ Автоматические email-уведомления
- ✅ Очереди сообщений с RabbitMQ
- ✅ Миграции базы данных
- ✅ Комплексное тестирование
- ✅ Корректное завершение работы
- ✅ Структурированное логирование

## Технологический стек

- **Язык**: Go 1.24.2
- **Коммуникация**: gRPC с Protocol Buffers
- **База данных**: PostgreSQL с драйвером pgx
- **Очереди сообщений**: RabbitMQ
- **Email**: SMTP (Gmail)
- **Миграции**: golang-migrate
- **Логирование**: Logrus
- **Тестирование**: Testify

## Предварительные требования

- Go 1.24.2 или выше
- PostgreSQL 12+
- RabbitMQ 3.8+
- Аккаунт Gmail с паролем приложения (для уведомлений)

## Установка

1. **Клонирование репозитория**
```bash
git clone https://github.com/ViktorOHJ/library-system.git
cd library-system
```

2. **Установка зависимостей**
```bash
go mod download
```

3. **Настройка переменных окружения**
Создайте файл `.env` в корневой директории:

```env
# URL баз данных
USR_DBURL=postgres://username:password@localhost/users_db?sslmode=disable
BOOKS_DBURL=postgres://username:password@localhost/books_db?sslmode=disable
LOANS_DBURL=postgres://username:password@localhost/loans_db?sslmode=disable
TEST_DBURL=postgres://username:password@localhost/test_db?sslmode=disable

# Порты сервисов
USERS_PORT=50051
BOOKS_PORT=50052
LOANS_PORT=50053
NOTIFICATIONS_PORT=50054

# Пути миграций
USERS_MIGRATIONS_PATH=file://users/migrations
BOOKS_MIGRATIONS_PATH=file://books/migrations
LOANS_MIGRATIONS_PATH=file://loans/migrations

# RabbitMQ
RABBIT_URL=amqp://guest:guest@localhost:5672/

# Конфигурация Email
EMAIL=your-email@gmail.com
MAIL_PASS=your-app-password
```

4. **Настройка баз данных**
Создайте необходимые базы данных PostgreSQL:
```sql
CREATE DATABASE users_db;
CREATE DATABASE books_db;
CREATE DATABASE loans_db;
CREATE DATABASE test_db;
```

5. **Настройка RabbitMQ**
Установите и запустите RabbitMQ, затем создайте необходимые обменники и очереди:
```bash
# Создание обменника
rabbitmqadmin declare exchange name=library type=direct

# Создание очередей
rabbitmqadmin declare queue name=borrow_queue
rabbitmqadmin declare queue name=return_queue

# Привязка очередей к обменнику
rabbitmqadmin declare binding source=library destination=borrow_queue routing_key=borrow_queue
rabbitmqadmin declare binding source=library destination=return_queue routing_key=return_queue
```

## Запуск сервисов

Запустите каждый сервис в отдельном терминале:

```bash
# Терминал 1 - Сервис пользователей
cd users
go run main.go db.go

# Терминал 2 - Сервис книг
cd books
go run main.go db.go

# Терминал 3 - Сервис займов
cd loans
go run main.go db.go

# Терминал 4 - Сервис уведомлений
cd notifications
go run main.go
```

## Схема базы данных

### Таблица пользователей
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL
);
```

### Таблица книг
```sql
CREATE TABLE books (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    author VARCHAR(100) NOT NULL,
    published_year INT NOT NULL,
    is_available BOOLEAN NOT NULL
);
```

### Таблица займов
```sql
CREATE TABLE loans (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    book_id INT NOT NULL,
    loan_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    return_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

## Примеры использования API

### Использование gRPC клиентов

```go
// Создание пользователя
userClient := userclient.NewUserClient("50051", 10*time.Second, logger)
user, err := userClient.Create(ctx, "Иван Иванов", "ivan@example.com")

// Создание книги
bookClient := bookclient.NewBookClient("50052", 10*time.Second, logger)
book, err := bookClient.Create(ctx, "Программирование на Go", "Алан Донован", 2024)

// Заимствование книги
loansClient := clients.NewLoansClient("50053", 10*time.Second, logger)
loan, err := loansClient.Borrow(ctx, "1", "1")

// Возврат книги
loan, err := loansClient.Return(ctx, "1")
```

## Тестирование

Запуск тестов для каждого сервиса:

```bash
# Тестирование всех сервисов
go test ./...

# Тестирование конкретного сервиса
cd users/server && go test -v
cd books/server && go test -v
cd loans/server && go test -v
cd notifications/server && go test -v
```

## Поток очередей сообщений

1. **Заимствование книги**: Сервис займов публикует сообщение в `borrow_queue`
2. **Возврат книги**: Сервис займов публикует сообщение в `return_queue`
3. **Уведомления**: Сервис потребляет сообщения и отправляет email

### Формат сообщения
```json
{
  "type": "Borrow",
  "user_name": "Иван Иванов",
  "book_title": "Программирование на Go",
  "book_author": "Алан Донован",
  "due_date": "2024-12-31",
  "loan_id": "123",
  "email": "ivan@example.com"
}
```

## Коммуникация между сервисами

```
┌─────────────┐    gRPC     ┌─────────────┐
│   Клиент    │────────────▶│  Сервис     │
└─────────────┘             │  займов     │
                            └─────┬───────┘
                                  │
                    ┌─────────────▼─────────────┐
                    │    gRPC вызовы к:         │
                    │  • Сервису пользователей  │
                    │  • Сервису книг           │
                    │  • Сервису уведомлений    │
                    └─────┬─────────────────────┘
                          │
                    ┌─────▼─────┐
                    │ RabbitMQ  │──────┐
                    └───────────┘      │
                                       │
                                ┌──────▼──────┐
                                │   Сервис    │
                                │уведомлений  │
                                └─────────────┘
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `USERS_PORT` | Порт сервиса пользователей | 50051 |
| `BOOKS_PORT` | Порт сервиса книг | 50052 |
| `LOANS_PORT` | Порт сервиса займов | 50053 |
| `NOTIFICATIONS_PORT` | Порт сервиса уведомлений | 50054 |
| `USR_DBURL` | Строка подключения к БД пользователей | - |
| `BOOKS_DBURL` | Строка подключения к БД книг | - |
| `LOANS_DBURL` | Строка подключения к БД займов | - |
| `TEST_DBURL` | Строка подключения к тестовой БД | - |
| `RABBIT_URL` | Строка подключения к RabbitMQ | - |
| `EMAIL` | SMTP email адрес | - |
| `MAIL_PASS` | Пароль приложения SMTP | - |

## Структура проекта

```
library-system/
├── books/
│   ├── client/           # gRPC клиент
│   ├── server/           # Реализация gRPC сервера
│   ├── migrations/       # Миграции базы данных
│   ├── main.go          # Точка входа сервиса
│   └── db.go            # Инициализация базы данных
├── users/
│   ├── client/
│   ├── server/
│   ├── migrations/
│   ├── main.go
│   └── db.go
├── loans/
│   ├── clients/         # Обертки клиентов для других сервисов
│   ├── server/
│   ├── migrations/
│   ├── main.go
│   └── db.go
├── notifications/
│   ├── client/
│   ├── server/
│   └── main.go
├── protos/              # Определения Protocol Buffer
├── rabbit/              # Клиент RabbitMQ
├── .env.example         # Шаблон переменных окружения
├── go.mod
└── README.md
```

## Безопасность

- Валидация входных данных на уровне gRPC
- Таймауты для всех внешних вызовов
- Корректная обработка ошибок
- Структурированное логирование для аудита
- Безопасное завершение работы сервисов

## Мониторинг и логи

Все сервисы используют структурированное логирование с Logrus:

```go
logger.WithFields(logrus.Fields{
    "user_id": userID,
    "book_id": bookID,
    "loan_id": loanID,
}).Info("Книга успешно заимствована")
```

## Возможности для расширения

- [ ] Аутентификация и авторизация JWT
- [ ] Резервирование книг
- [ ] Система штрафов за просрочку
- [ ] REST API шлюз
- [ ] Метрики Prometheus
- [ ] Трассировка с Jaeger
- [ ] Кеширование Redis
- [ ] Поиск по каталогу книг

## Лицензия

MIT License

## Автор

ViktorOHJ