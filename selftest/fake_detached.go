package selftest

import "reflect"

// Public projections retain value fields that are deliberately absent from JSON.
func detached[T any](v T) T { return cloneValue(reflect.ValueOf(v)).Interface().(T) }
func cloneValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		r := reflect.New(v.Type().Elem())
		r.Elem().Set(cloneValue(v.Elem()))
		return r
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		r := reflect.New(v.Type()).Elem()
		r.Set(cloneValue(v.Elem()))
		return r
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		r := reflect.MakeMapWithSize(v.Type(), v.Len())
		it := v.MapRange()
		for it.Next() {
			r.SetMapIndex(it.Key(), cloneValue(it.Value()))
		}
		return r
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		r := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			r.Index(i).Set(cloneValue(v.Index(i)))
		}
		return r
	case reflect.Struct:
		r := reflect.New(v.Type()).Elem()
		r.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if r.Field(i).CanSet() && v.Type().Field(i).IsExported() {
				r.Field(i).Set(cloneValue(v.Field(i)))
			}
		}
		return r
	default:
		return v
	}
}
