package hh

import (
	"fmt"
	"reflect"
	"time"
)

// diffIgnored are the fields that the text of a hand does not carry: whether an
// action was inferred by Repair, whether the result was worked out from the
// actions, and the lines the parser did not understand. Every other field is
// compared, the pot and rake printed as each street begins included.
var diffIgnored = map[string]bool{"Inferred": true, "Completed": true, "Unknown": true}

// Diff compares the hand with g, field by field, and returns "" if they are
// identical. Otherwise it returns the path of the first field that differs with
// the two values, such as "Players[2].Stack: 5000 != 5100". It is how a hand
// written by String is shown to read back as the same hand. The fields in
// diffIgnored are not compared.
func (h *HandHistory) Diff(g *HandHistory) string {
	return diffValue("", reflect.ValueOf(h).Elem(), reflect.ValueOf(g).Elem())
}

func diffValue(path string, a, b reflect.Value) string {
	switch a.Kind() {
	case reflect.Struct:
		if a.Type() == reflect.TypeOf(time.Time{}) {
			if x, y := a.Interface().(time.Time), b.Interface().(time.Time); !x.Equal(y) {
				return fmt.Sprintf("%s: %v != %v", path, x, y)
			}
			return ""
		}
		for i := 0; i < a.NumField(); i++ {
			name := a.Type().Field(i).Name
			if diffIgnored[name] {
				continue
			}
			if d := diffValue(path+"."+name, a.Field(i), b.Field(i)); d != "" {
				return d
			}
		}
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return fmt.Sprintf("%s: length %d != %d", path, a.Len(), b.Len())
		}
		for i := 0; i < a.Len(); i++ {
			if d := diffValue(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i)); d != "" {
				return d
			}
		}
	case reflect.String:
		if a.String() != b.String() {
			return fmt.Sprintf("%s: %q != %q", path, a.String(), b.String())
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			return fmt.Sprintf("%s: %v != %v", path, a.Bool(), b.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			return fmt.Sprintf("%s: %d != %d", path, a.Int(), b.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if a.Uint() != b.Uint() {
			return fmt.Sprintf("%s: %d != %d", path, a.Uint(), b.Uint())
		}
	}
	return ""
}
