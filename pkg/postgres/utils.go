package postgres

import (
	"errors"
	"fmt"
	"reflect"
)

// Вспомогательная функция для создания слайса структур
func StructFieldsPointers(strct interface{}, columns []string) ([]interface{}, error) {
	v := reflect.ValueOf(strct)

	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, errors.New("input must be a pointer to a struct")
	}

	fieldMap := make(map[string]reflect.Value)
	CollectFields(v.Elem(), "", fieldMap)

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

// CollectFields recursively collects fields, including nested structs
func CollectFields(v reflect.Value, prefix string, fieldMap map[string]reflect.Value) {
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
				CollectFields(fieldValue, prefix, fieldMap)
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
			CollectFields(fieldValue, colName, fieldMap)
		} else {
			fieldMap[colName] = fieldValue
		}
	}
}

// GetColumnNames - рекурсивная функция для получения списка имен колонок из структуры
func GetColumnNames(dest interface{}) ([]string, error) {
	var columns []string
	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
		return nil, errors.New("dest must be a pointer to a struct")
	}
	CollectColumnNames(destVal.Elem(), "", &columns)
	return columns, nil
}

// CollectColumnNames recursively collects column names from the struct fields
func CollectColumnNames(v reflect.Value, prefix string, columns *[]string) {
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
				CollectColumnNames(fieldValue, prefix, columns)
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
			CollectColumnNames(fieldValue, colName, columns)
		} else {
			*columns = append(*columns, colName)
		}
	}
}
