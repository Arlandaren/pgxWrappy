package postgres

import (
	"context"
	"errors"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

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
	columns, err := GetColumnNames(dest)
	if err != nil {
		return err
	}

	// Получаем указатели на поля структуры
	fields, err := StructFieldsPointers(dest, columns)
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
func (tw *TxWrapper) Select(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error {
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

		fields, err := StructFieldsPointers(elemPtr.Interface(), columns)
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
