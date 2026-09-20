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

// getPrivateField reads an unexported field so a test can then pass it to
// setPrivateField. A plain field.Interface() panic on unexported fields, so the
// address is re-typed through unsafe exactly like setPrivateField does.
func getPrivateField(target any, fieldName string) any {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		panic("getPrivateField requires a non-nil pointer")
	}
	field := rv.Elem().FieldByName(fieldName)
	if !field.IsValid() {
		panic("getPrivateField: field not found: " + fieldName)
	}
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}
