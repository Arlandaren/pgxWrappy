package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

var (
	timeType    = reflect.TypeOf(time.Time{})
	scannerType = reflect.TypeOf((*sql.Scanner)(nil)).Elem()
)

// isLeafType reports whether a struct-kind field should be scanned directly
// into a single column instead of having its own fields flattened into
// separate columns. Without this check, types such as time.Time or
// pgtype.Timestamptz (which are structs under the hood) would be
// incorrectly treated as nested "joined" structs, and none of the columns
// they should occupy would ever be found.
func isLeafType(t reflect.Type) bool {
	if t == timeType {
		return true
	}
	if reflect.PointerTo(t).Implements(scannerType) {
		return true
	}
	return false
}

// fieldPaths holds, for a given struct type, the ordered column names and
// the reflect.Value.FieldByIndex path used to reach each one. It is built
// once per type and cached, since the layout only depends on the type's
// fields and tags, not on any particular instance.
type fieldPaths struct {
	columns []string
	byName  map[string][]int
}

var typeCache sync.Map // map[reflect.Type]*fieldPaths

func getFieldPaths(t reflect.Type) *fieldPaths {
	if cached, ok := typeCache.Load(t); ok {
		return cached.(*fieldPaths)
	}

	fp := &fieldPaths{byName: make(map[string][]int)}
	collectFieldPaths(t, "", nil, fp)

	actual, _ := typeCache.LoadOrStore(t, fp)
	return actual.(*fieldPaths)
}

func collectFieldPaths(t reflect.Type, prefix string, index []int, fp *fieldPaths) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.PkgPath != "" {
			// unexported field, cannot be set via reflection
			continue
		}

		curIndex := make([]int, len(index)+1)
		copy(curIndex, index)
		curIndex[len(index)] = i

		fieldType := field.Type
		isNestedStruct := fieldType.Kind() == reflect.Struct && !isLeafType(fieldType)

		tag := field.Tag.Get("db")
		if tag == "-" {
			if isNestedStruct {
				// Process inner fields without adding a prefix
				collectFieldPaths(fieldType, prefix, curIndex, fp)
			}
			// Skip processing if it's not a struct we should flatten
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

		if isNestedStruct {
			collectFieldPaths(fieldType, colName, curIndex, fp)
		} else {
			fp.columns = append(fp.columns, colName)
			fp.byName[colName] = curIndex
		}
	}
}

// StructFieldsPointers is a helper function to create a slice of pointers to the struct's fields
func StructFieldsPointers(strct interface{}, columns []string) ([]interface{}, error) {
	v := reflect.ValueOf(strct)

	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, errors.New("input must be a pointer to a struct")
	}

	elem := v.Elem()
	fp := getFieldPaths(elem.Type())

	fields := make([]interface{}, len(columns))
	for i, col := range columns {
		index, ok := fp.byName[col]
		if !ok {
			return nil, fmt.Errorf("no matching struct field found for column %s", col)
		}
		fields[i] = elem.FieldByIndex(index).Addr().Interface()
	}

	return fields, nil
}

// GetColumnNames is a recursive function to get a list of column names from the struct
func GetColumnNames(dest interface{}) ([]string, error) {
	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
		return nil, errors.New("dest must be a pointer to a struct")
	}

	fp := getFieldPaths(destVal.Elem().Type())
	columns := make([]string, len(fp.columns))
	copy(columns, fp.columns)
	return columns, nil
}

// CollectFields recursively collects fields, including nested structs.
//
// Get and Select no longer call this directly (they use a cached,
// type-based field resolver instead), but it is kept with its original
// signature for backward compatibility with any external callers.
func CollectFields(v reflect.Value, prefix string, fieldMap map[string]reflect.Value) {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		isNestedStruct := fieldValue.Kind() == reflect.Struct && !isLeafType(fieldValue.Type())

		tag := field.Tag.Get("db")
		if tag == "-" {
			if isNestedStruct {
				// Process inner fields without adding prefix
				CollectFields(fieldValue, prefix, fieldMap)
			}
			// Skip processing if it's not a struct we should flatten
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

		if isNestedStruct {
			CollectFields(fieldValue, colName, fieldMap)
		} else {
			fieldMap[colName] = fieldValue
		}
	}
}

// CollectColumnNames recursively collects column names from the struct fields.
//
// GetColumnNames no longer calls this directly (see CollectFields), but it
// is kept with its original signature for backward compatibility with any
// external callers.
func CollectColumnNames(v reflect.Value, prefix string, columns *[]string) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		isNestedStruct := fieldValue.Kind() == reflect.Struct && !isLeafType(fieldValue.Type())

		tag := field.Tag.Get("db")
		if tag == "-" {
			if isNestedStruct {
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

		if isNestedStruct {
			CollectColumnNames(fieldValue, colName, columns)
		} else {
			*columns = append(*columns, colName)
		}
	}
}
