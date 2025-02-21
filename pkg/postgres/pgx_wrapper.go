package postgres

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB - интерфейс для обёртки
type DB interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// Wrapper - структура, которая содержит пул соединений с базой данных
type Wrapper struct {
	pool *pgxpool.Pool
}

// NewWrapper - функция для создания новой обёртки с пулом соединений
func NewWrapper(pool *pgxpool.Pool) *Wrapper {
	return &Wrapper{pool: pool}
}

// QueryRow - обёртка для метода QueryRow
func (w *Wrapper) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return w.pool.QueryRow(ctx, sql, args...)
}

// Query - обёртка для метода Query
func (w *Wrapper) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return w.pool.Query(ctx, sql, args...)
}

// Exec - обёртка для метода Exec
func (w *Wrapper) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return w.pool.Exec(ctx, sql, args...)
}

// Get - функция для выполнения запроса, который возвращает одну строку, и сканирования ее в переданную структуру
func (w *Wrapper) Get(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error {
    destVal := reflect.ValueOf(dest)
    if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
        return errors.New("dest must be a pointer to a struct")
    }

    // Получаем ожидаемые имена колонок из структуры назначения
    columns, err := getColumnNames(dest)
    if err != nil {
        return err
    }

    // Получаем указатели на поля структуры
    fields, err := structFieldsPointers(dest, columns)
    if err != nil {
        return err
    }

    // Выполняем запрос
    row := w.pool.QueryRow(ctx, sqlStr, args...)

    // Считываем данные в поля структуры
    if err := row.Scan(fields...); err != nil {
        return err
    }

    return nil
}


// Select - функция для получения нескольких результатов и их сканирования в слайс
func (w*Wrapper) Select(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error {
    rows, err := w.pool.Query(ctx, sqlStr, args...)
    if err != nil {
        return err
    }
    defer rows.Close()

    destVal := reflect.ValueOf(dest)
    if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Slice {
        return errors.New("dest must be a pointer to a slice")
    }

    sliceVal := destVal.Elem()
    elemType := sliceVal.Type().Elem()

    ptrToStruct := false
    if elemType.Kind() == reflect.Ptr && elemType.Elem().Kind() == reflect.Struct {
        ptrToStruct = true
        elemType = elemType.Elem()
    } else if elemType.Kind() != reflect.Struct {
        return errors.New("slice elements must be structs or pointers to structs")
    }

    fieldDescriptions := rows.FieldDescriptions()
    columns := make([]string, len(fieldDescriptions))
    for i, fd := range fieldDescriptions {
        columns[i] = string(fd.Name)
    }

    for rows.Next() {
        elemPtr := reflect.New(elemType)

        fields, err := structFieldsPointers(elemPtr.Interface(), columns)
        if err != nil {
            return err
        }

        if err := rows.Scan(fields...); err != nil {
            return err
        }

        if ptrToStruct {
            sliceVal.Set(reflect.Append(sliceVal, elemPtr))
        } else {
            sliceVal.Set(reflect.Append(sliceVal, elemPtr.Elem()))
        }
    }

    if err := rows.Err(); err != nil {
        return err
    }

    return nil
}

// Begin - метод для начала транзакции
func (w *Wrapper) Begin(ctx context.Context) (*TxWrapper, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &TxWrapper{tx: tx}, nil
}

// BeginTx - метод для начала транзакции с опциями
func (w *Wrapper) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (*TxWrapper, error) {
	tx, err := w.pool.BeginTx(ctx, txOptions)
	if err != nil {
		return nil, err
	}
	return &TxWrapper{tx: tx}, nil
}

// TxWrapper - обёртка для транзакций
type TxWrapper struct {
	tx pgx.Tx
}

// QueryRow - обёртка для метода QueryRow в транзакции
func (tw *TxWrapper) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return tw.tx.QueryRow(ctx, sql, args...)
}

// Query - обёртка для метода Query в транзакции
func (tw *TxWrapper) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return tw.tx.Query(ctx, sql, args...)
}

// Exec - обёртка для метода Exec в транзакции
func (tw *TxWrapper) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return tw.tx.Exec(ctx, sql, args...)
}

// Get - функция для выполнения запроса в транзакции, который возвращает одну строку, и сканирования ее в структуру
func (tw *TxWrapper) Get(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error {
    destVal := reflect.ValueOf(dest)
    if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
        return errors.New("dest must be a pointer to a struct")
    }

    // Получаем ожидаемые имена колонок из структуры назначения
    columns, err := getColumnNames(dest)
    if err != nil {
        return err
    }

    // Получаем указатели на поля структуры
    fields, err := structFieldsPointers(dest, columns)
    if err != nil {
        return err
    }

    // Выполняем запрос
    row := tw.tx.QueryRow(ctx, sqlStr, args...)

    // Считываем данные в поля структуры
    if err := row.Scan(fields...); err != nil {
        return err
    }

    return nil
}



// Select - функция для получения нескольких результатов в транзакции и их сканирования в слайс
func (tw*TxWrapper) Select(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error {
    rows, err := tw.tx.Query(ctx, sqlStr, args...)
    if err != nil {
        return err
    }
    defer rows.Close()

    destVal := reflect.ValueOf(dest)
    if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Slice {
        return errors.New("dest must be a pointer to a slice")
    }

    sliceVal := destVal.Elem()
    elemType := sliceVal.Type().Elem()

    ptrToStruct := false
    if elemType.Kind() == reflect.Ptr && elemType.Elem().Kind() == reflect.Struct {
        ptrToStruct = true
        elemType = elemType.Elem()
    } else if elemType.Kind() != reflect.Struct {
        return errors.New("slice elements must be structs or pointers to structs")
    }

    fieldDescriptions := rows.FieldDescriptions()
    columns := make([]string, len(fieldDescriptions))
    for i, fd := range fieldDescriptions {
        columns[i] = string(fd.Name)
    }

    for rows.Next() {
        elemPtr := reflect.New(elemType)

        fields, err := structFieldsPointers(elemPtr.Interface(), columns)
        if err != nil {
            return err
        }

        if err := rows.Scan(fields...); err != nil {
            return err
        }

        if ptrToStruct {
            sliceVal.Set(reflect.Append(sliceVal, elemPtr))
        } else {
            sliceVal.Set(reflect.Append(sliceVal, elemPtr.Elem()))
        }
    }

    if err := rows.Err(); err != nil {
        return err
    }

    return nil
}

// Commit - метод для фиксации транзакции
func (tw *TxWrapper) Commit(ctx context.Context) error {
	return tw.tx.Commit(ctx)
}

// Rollback - метод для отката транзакции
func (tw *TxWrapper) Rollback(ctx context.Context) error {
	return tw.tx.Rollback(ctx)
}

// Вспомогательная функция для создания слайса структур
func structFieldsPointers(strct interface{}, columns []string) ([]interface{}, error) {
    v := reflect.ValueOf(strct)

    if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
        return nil, errors.New("input must be a pointer to a struct")
    }

    fieldMap := make(map[string]reflect.Value)
    collectFields(v.Elem(), "", fieldMap)

    fields := make([]interface{}, len(columns))
    for i, col := range columns {
        fieldVal, ok := fieldMap[col]
        if !ok {
            return nil, fmt.Errorf("no matching struct field found for column %s", col)
        }
        fields[i] = fieldVal.Addr().Interface()
    }

    return fields, nil
}

// collectFields recursively collects fields, including nested structs
func collectFields(v reflect.Value, prefix string, fieldMap map[string]reflect.Value) {
    t := v.Type()

    for i := 0; i < v.NumField(); i++ {
        field := t.Field(i)
        fieldValue := v.Field(i)

        if !fieldValue.CanSet() {
            continue
        }

        tag := field.Tag.Get("db")
        if tag == "-" {
            if fieldValue.Kind() == reflect.Struct {
                // Process inner fields without adding prefix
                collectFields(fieldValue, prefix, fieldMap)
            }
            // Skip processing if it's not a struct
            continue
        }
        if tag == "" {
            tag = field.Name
        }

        var colName string
        if prefix != "" && !field.Anonymous {
            colName = prefix + "_" + tag
        } else {
            colName = tag
        }

        if fieldValue.Kind() == reflect.Struct {
            collectFields(fieldValue, colName, fieldMap)
        } else {
            fieldMap[colName] = fieldValue
        }
    }
}

// getColumnNames - рекурсивная функция для получения списка имен колонок из структуры
func getColumnNames(dest interface{}) ([]string, error) {
    var columns []string
    destVal := reflect.ValueOf(dest)
    if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
        return nil, errors.New("dest must be a pointer to a struct")
    }
    collectColumnNames(destVal.Elem(), "", &columns)
    return columns, nil
}

// collectColumnNames recursively collects column names from the struct fields
func collectColumnNames(v reflect.Value, prefix string, columns*[]string) {
    t := v.Type()
    for i := 0; i < v.NumField(); i++ {
        field := t.Field(i)
        fieldValue := v.Field(i)

        if !fieldValue.CanSet() {
            continue
        }

        tag := field.Tag.Get("db")
        if tag == "-" {
            if fieldValue.Kind() == reflect.Struct {
                // Collect names from inner struct without adding prefix
                collectColumnNames(fieldValue, prefix, columns)
            }
            continue
        }
        if tag == "" {
            tag = field.Name
        }

        var colName string
        if prefix != "" && !field.Anonymous {
            colName = prefix + "_" + tag
        } else {
            colName = tag
        }

        if fieldValue.Kind() == reflect.Struct {
            collectColumnNames(fieldValue, colName, columns)
        } else {
            *columns = append(*columns, colName)
        }
    }
}
