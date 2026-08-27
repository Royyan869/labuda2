package serverboot

import (
	"reflect"
	"unsafe"
)

func setPrivateField(target any, fieldName string, value any) {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		panic("setPrivateField requires a non-nil pointer")
	}
	elem := rv.Elem()
	field := elem.FieldByName(fieldName)
	if !field.IsValid() {
		panic("setPrivateField: field not found: " + fieldName)
	}
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}
