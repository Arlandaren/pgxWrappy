package postgres

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

type flatStruct struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

type untaggedStruct struct {
	ID        int
	FirstName string
}

type addressStruct struct {
	Street string `db:"street"`
	City   string `db:"city"`
}

type nestedDashStruct struct {
	ID      int           `db:"id"`
	Address addressStruct `db:"-"`
}

type nestedNamedStruct struct {
	ID      int           `db:"id"`
	Address addressStruct `db:"address"`
}

type withTimeStruct struct {
	ID        int       `db:"id"`
	CreatedAt time.Time `db:"created_at"`
}

type withUnexportedStruct struct {
	ID     int `db:"id"`
	hidden string
}

// mockScanner is a struct-kind type that implements sql.Scanner, mimicking
// pgx pgtype.* types (e.g. pgtype.Timestamptz), which are structs under the
// hood but must be scanned as a single leaf column rather than flattened.
type mockScanner struct {
	Value string
}

func (m *mockScanner) Scan(src interface{}) error {
	s, _ := src.(string)
	m.Value = s
	return nil
}

type withScannerStruct struct {
	ID   int         `db:"id"`
	Meta mockScanner `db:"meta"`
}

func sortedCopy(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}

func TestGetColumnNames_Flat(t *testing.T) {
	columns, err := GetColumnNames(&flatStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(columns, []string{"id", "name"}) {
		t.Fatalf("unexpected columns: %v", columns)
	}
}

func TestGetColumnNames_UntaggedFieldsUseFieldName(t *testing.T) {
	columns, err := GetColumnNames(&untaggedStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(columns, []string{"ID", "FirstName"}) {
		t.Fatalf("unexpected columns: %v", columns)
	}
}

func TestGetColumnNames_NestedDashTagFlattensWithoutPrefix(t *testing.T) {
	columns, err := GetColumnNames(&nestedDashStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(columns, []string{"id", "street", "city"}) {
		t.Fatalf("unexpected columns: %v", columns)
	}
}

func TestGetColumnNames_NestedNamedTagAddsPrefix(t *testing.T) {
	columns, err := GetColumnNames(&nestedNamedStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(columns, []string{"id", "address_street", "address_city"}) {
		t.Fatalf("unexpected columns: %v", columns)
	}
}

func TestGetColumnNames_UnexportedFieldsAreSkipped(t *testing.T) {
	columns, err := GetColumnNames(&withUnexportedStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(columns, []string{"id"}) {
		t.Fatalf("unexpected columns: %v", columns)
	}
}

func TestGetColumnNames_ErrorsOnNonPointerOrNonStruct(t *testing.T) {
	if _, err := GetColumnNames(flatStruct{}); err == nil {
		t.Fatal("expected error for non-pointer argument")
	}
	notStruct := 5
	if _, err := GetColumnNames(&notStruct); err == nil {
		t.Fatal("expected error for pointer to non-struct argument")
	}
}

// Regression test: previously, any struct-kind field (including time.Time)
// was recursed into as if it were a nested "joined" struct. Since time.Time
// has only unexported internal fields, this meant the "created_at" column
// was silently dropped and Get/Select failed with
// "no matching struct field found for column created_at".
func TestTimeTimeFieldIsTreatedAsLeafColumn(t *testing.T) {
	columns, err := GetColumnNames(&withTimeStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(columns, []string{"id", "created_at"}) {
		t.Fatalf("unexpected columns: %v", columns)
	}

	dest := &withTimeStruct{}
	fields, err := StructFieldsPointers(dest, columns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("expected 2 field pointers, got %d", len(fields))
	}
	if _, ok := fields[1].(*time.Time); !ok {
		t.Fatalf("expected second field to be *time.Time, got %T", fields[1])
	}
}

// Regression test: a struct field whose type implements sql.Scanner (as
// pgx's pgtype.* types do) must also be treated as a single leaf column.
func TestScannerFieldIsTreatedAsLeafColumn(t *testing.T) {
	columns, err := GetColumnNames(&withScannerStruct{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(columns, []string{"id", "meta"}) {
		t.Fatalf("unexpected columns: %v", columns)
	}

	dest := &withScannerStruct{}
	fields, err := StructFieldsPointers(dest, columns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := fields[1].(*mockScanner); !ok {
		t.Fatalf("expected second field to be *mockScanner, got %T", fields[1])
	}
}

func TestStructFieldsPointers_ReturnsAddressableFields(t *testing.T) {
	dest := &flatStruct{}
	fields, err := StructFieldsPointers(dest, []string{"id", "name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	*(fields[0].(*int)) = 42
	*(fields[1].(*string)) = "alice"

	if dest.ID != 42 || dest.Name != "alice" {
		t.Fatalf("scanning through returned pointers did not mutate dest: %+v", dest)
	}
}

func TestStructFieldsPointers_ErrorOnUnknownColumn(t *testing.T) {
	dest := &flatStruct{}
	_, err := StructFieldsPointers(dest, []string{"id", "does_not_exist"})
	if err == nil {
		t.Fatal("expected error for unknown column")
	}
}

func TestStructFieldsPointers_ErrorOnNonPointerOrNonStruct(t *testing.T) {
	if _, err := StructFieldsPointers(flatStruct{}, []string{"id"}); err == nil {
		t.Fatal("expected error for non-pointer argument")
	}
	notStruct := 5
	if _, err := StructFieldsPointers(&notStruct, []string{"id"}); err == nil {
		t.Fatal("expected error for pointer to non-struct argument")
	}
}

// CollectFields/CollectColumnNames are kept for backward compatibility; make
// sure they still behave correctly (including the leaf-type fix) when
// called directly.
func TestCollectFields_DirectCallMatchesGetColumnNames(t *testing.T) {
	dest := &withTimeStruct{}
	fieldMap := make(map[string]reflect.Value)
	CollectFields(reflect.ValueOf(dest).Elem(), "", fieldMap)

	got := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		got = append(got, k)
	}

	want := []string{"id", "created_at"}
	if !reflect.DeepEqual(sortedCopy(got), sortedCopy(want)) {
		t.Fatalf("unexpected fields collected: %v", got)
	}
}

func TestCollectColumnNames_DirectCallMatchesGetColumnNames(t *testing.T) {
	dest := &nestedNamedStruct{}
	var columns []string
	CollectColumnNames(reflect.ValueOf(dest).Elem(), "", &columns)

	if !reflect.DeepEqual(columns, []string{"id", "address_street", "address_city"}) {
		t.Fatalf("unexpected columns: %v", columns)
	}
}
