package postgres

import (
	"context"
	"errors"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TxWrapper is a wrapper for transactions
type TxWrapper struct {
	Tx pgx.Tx
}

// QueryRow is a wrapper for the QueryRow method within a transaction
func (tw *TxWrapper) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return tw.Tx.QueryRow(ctx, sql, args...)
}

// Query is a wrapper for the Query method within a transaction
func (tw *TxWrapper) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return tw.Tx.Query(ctx, sql, args...)
}

// Exec is a wrapper for the Exec method within a transaction
func (tw *TxWrapper) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return tw.Tx.Exec(ctx, sql, args...)
}

// Get executes a query within a transaction that returns one row and scans it into a struct
func (tw *TxWrapper) Get(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error {
	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
		return errors.New("dest must be a pointer to a struct")
	}

	// Get expected column names from the destination struct
	columns, err := GetColumnNames(dest)
	if err != nil {
		return err
	}

	// Get pointers to the struct fields
	fields, err := StructFieldsPointers(dest, columns)
	if err != nil {
		return err
	}

	// Execute the query
	row := tw.Tx.QueryRow(ctx, sqlStr, args...)

	// Scan the data into the struct fields
	if err := row.Scan(fields...); err != nil {
		return err
	}

	return nil
}

// Select retrieves multiple results within a transaction and scans them into a slice
func (tw *TxWrapper) Select(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error {
	rows, err := tw.Tx.Query(ctx, sqlStr, args...)
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

// Commit commits the transaction
func (tw *TxWrapper) Commit(ctx context.Context) error {
	return tw.Tx.Commit(ctx)
}

// Rollback rolls back the transaction
func (tw *TxWrapper) Rollback(ctx context.Context) error {
	return tw.Tx.Rollback(ctx)
}
