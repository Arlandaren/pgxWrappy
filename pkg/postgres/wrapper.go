package postgres

import (
	"context"
	"errors"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is an interface for the database wrapper
type DB interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// Wrapper is a structure that contains a connection pool to the database
type Wrapper struct {
	Pool *pgxpool.Pool
}

// NewWrapper creates a new wrapper with a connection pool
func NewWrapper(pool *pgxpool.Pool) *Wrapper {
	return &Wrapper{Pool: pool}
}

// QueryRow is a wrapper for the QueryRow method
func (w *Wrapper) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return w.Pool.QueryRow(ctx, sql, args...)
}

// Query is a wrapper for the Query method
func (w *Wrapper) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return w.Pool.Query(ctx, sql, args...)
}

// Exec is a wrapper for the Exec method
func (w *Wrapper) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return w.Pool.Exec(ctx, sql, args...)
}

// Get executes a query that returns one row and scans it into the passed-in struct
func (w *Wrapper) Get(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error {
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
	row := w.Pool.QueryRow(ctx, sqlStr, args...)

	// Scan the data into the struct fields
	if err := row.Scan(fields...); err != nil {
		return err
	}

	return nil
}

// Select retrieves multiple results and scans them into a slice
func (w *Wrapper) Select(ctx context.Context, dest interface{}, sqlStr string, args ...interface{}) error {
	rows, err := w.Pool.Query(ctx, sqlStr, args...)
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

// Begin starts a transaction
func (w *Wrapper) Begin(ctx context.Context) (*TxWrapper, error) {
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &TxWrapper{Tx: tx}, nil
}

// BeginTx starts a transaction with options
func (w *Wrapper) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (*TxWrapper, error) {
	tx, err := w.Pool.BeginTx(ctx, txOptions)
	if err != nil {
		return nil, err
	}
	return &TxWrapper{Tx: tx}, nil
}
