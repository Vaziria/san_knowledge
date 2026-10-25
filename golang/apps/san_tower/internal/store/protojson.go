package store

import (
	"context"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm/schema"
)

// A column tagged `gorm:"serializer:protojson"` holds a proto message (a
// pointer field) as protojson text: readable in any SQLite browser, and
// json_* functions work on it. Reading accepts field names in either form
// (sourceSocket or source_socket) and skips unknown fields, so older rows
// still load.
func init() {
	schema.RegisterSerializer("protojson", protoJSON{})
}

type protoJSON struct{}

func (protoJSON) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	t := field.FieldType
	if t.Kind() != reflect.Pointer {
		return fmt.Errorf("protojson: field %s is not a pointer to a message", field.Name)
	}
	m, ok := reflect.New(t.Elem()).Interface().(proto.Message)
	if !ok {
		return fmt.Errorf("protojson: field %s is not a message", field.Name)
	}
	var data []byte
	switch v := dbValue.(type) {
	case nil:
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("protojson: field %s: cannot read a %T", field.Name, dbValue)
	}
	if len(data) > 0 {
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, m); err != nil {
			return fmt.Errorf("protojson: field %s: %w", field.Name, err)
		}
	}
	field.ReflectValueOf(ctx, dst).Set(reflect.ValueOf(m))
	return nil
}

func (protoJSON) Value(_ context.Context, field *schema.Field, _ reflect.Value, fieldValue any) (any, error) {
	m, _ := fieldValue.(proto.Message)
	if m == nil || reflect.ValueOf(m).IsNil() {
		return "{}", nil
	}
	b, err := protojson.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("protojson: field %s: %w", field.Name, err)
	}
	return string(b), nil
}
