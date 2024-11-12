# Пакет `pgxWrappy`

Пакет `pgxWrappy` предоставляет удобную обёртку над библиотекой [pgx](https://github.com/jackc/pgx) для работы с базой данных PostgreSQL. Он упрощает выполнение SQL-запросов, обработку результатов и управление транзакциями, предоставляя высокоуровневые функции для взаимодействия с базой данных.

## Содержание

- [Установка](#установка)
- [Использование](#использование)
- [Структура `Wrapper`](#структура-wrapper)
- [Доступные методы](#доступные-методы)
    - [`QueryRow`](#queryrow)
    - [`Query`](#query)
    - [`Exec`](#exec)
    - [`Get`](#get)
    - [`Select`](#select)
- [Транзакции](#транзакции)
    - [Начало транзакции](#начало-транзакции)
    - [Методы транзакции](#методы-транзакции)
- [Примеры использования](#примеры-использования)
- [Обратная связь](#обратная-связь)

## Установка

Используйте `go get` для установки пакета:

```bash
go get -u github.com/Arlandaren/pgxWrappy/postgres
```

## Использование

Импортируйте пакет в ваш проект:

```go
import "github.com/Arlandaren/pgxWrappy/postgres"
```

## Структура `Wrapper`

Основной структурой пакета является `Wrapper`, которая содержит пул соединений с базой данных и предоставляет методы для выполнения запросов.

### Создание обёртки

```go
// Инициализация пула соединений
config, err := pgxpool.ParseConfig(connectionString)
if err != nil {
    // обработка ошибки
}

pool, err := pgxpool.NewWithConfig(context.Background(), config)
if err != nil {
    // обработка ошибки
}

// Создание обёртки
db := postgres.NewWrapper(pool)
```

## Доступные методы

### `QueryRow`

Выполняет запрос, который ожидает одну строку результата. Возвращает объект `pgx.Row`.

```go
func (w *Wrapper) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
```

**Пример использования:**

```go
row := db.QueryRow(ctx, "SELECT name FROM users WHERE id=$1", userID)
var name string
err := row.Scan(&name)
if err != nil {
    // обработка ошибки
}
```

### `Query`

Выполняет запрос, который может возвращать несколько строк. Возвращает `pgx.Rows`.

```go
func (w*Wrapper) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
```

**Пример использования:**

```go
rows, err := db.Query(ctx, "SELECT id, name FROM users")
if err != nil {
    // обработка ошибки
}
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    err := rows.Scan(&id, &name)
    if err != nil {
        // обработка ошибки
    }
    // обработка данных
}
```

### `Exec`

Выполняет команду, не возвращающую строк (например, `INSERT`, `UPDATE`, `DELETE`).

```go
func (w *Wrapper) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
```

**Пример использования:**

```go
_, err := db.Exec(ctx, "UPDATE users SET name=$1 WHERE id=$2", newName, userID)
if err != nil {
    // обработка ошибки
}
```

### `Get`

Выполняет запрос, ожидающий одну строку результата, и сканирует данные в переданную структуру.

```go
func (w*Wrapper) Get(ctx context.Context, dest interface{}, sql string, args ...interface{}) error
```

**Пример использования:**

```go
type User struct {
    ID   int
    Name string
}

var user User
err := db.Get(ctx, &user, "SELECT id, name FROM users WHERE id=$1", userID)
if err != nil {
    // обработка ошибки
}
```

### `Select`

Выполняет запрос, который может возвращать несколько строк, и сканирует данные в слайс структур.

```go
func (w *Wrapper) Select(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error
```

**Пример использования:**

```go
var users []User
err := db.Select(ctx, &users, "SELECT id, name FROM users")
if err != nil {
    // обработка ошибки
}
```

## Транзакции

Пакет предоставляет обёртку для транзакций через структуру `TxWrapper`.

### Начало транзакции

Начать транзакцию можно с помощью метода `Begin`:

```go
func (w*Wrapper) Begin(ctx context.Context) (*TxWrapper, error)
```

Или с опциями транзакции:

```go
func (w*Wrapper) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (*TxWrapper, error)
```

**Пример использования:**

```go
tx, err := db.Begin(ctx)
if err != nil {
    // обработка ошибки
}
defer tx.Rollback(ctx)

// Выполнение операций в транзакции

if err := tx.Commit(ctx); err != nil {
    // обработка ошибки
}
```

### Методы транзакции

`TxWrapper` имеет такие же методы, что и `Wrapper`, для выполнения запросов внутри транзакции:

- `QueryRow`
- `Query`
- `Exec`
- `Get`
- `Select`
- `Commit`
- `Rollback`

**Пример использования методов транзакции:**

```go
// В рамках транзакции
var user User
err := tx.Get(ctx, &user, "SELECT id, name FROM users WHERE id=$1", userID)
if err != nil {
    // обработка ошибки
}

// Завершение транзакции
if err := tx.Commit(ctx); err != nil {
    // обработка ошибки
}
```

## Примеры использования

### Вставка данных с использованием транзакции

```go
tx, err := db.Begin(ctx)
if err != nil {
    // обработка ошибки
}
defer tx.Rollback(ctx)

_, err = tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "Alice")
if err != nil {
    // обработка ошибки
}

if err := tx.Commit(ctx); err != nil {
    // обработка ошибки
}
```

### Получение списка пользователей

```go
var users []User
err := db.Select(ctx, &users, "SELECT id, name FROM users")
if err != nil {
    // обработка ошибки
}

for_, user := range users {
    fmt.Printf("User ID: %d, Name: %s\n", user.ID, user.Name)
}
```

### Обработка одной строки результата

```go
var user User
err := db.Get(ctx, &user, "SELECT id, name FROM users WHERE id=$1", 1)
if err != nil {
    if errors.Is(err, pgx.ErrNoRows) {
        fmt.Println("Пользователь не найден")
    } else {
        // обработка ошибки
    }
} else {
    fmt.Printf("User ID: %d, Name: %s\n", user.ID, user.Name)
}
```


## Обратная связь

Если у вас есть вопросы или предложения, пожалуйста, создайте issue или отправьте pull request в репозитории GitHub.

---