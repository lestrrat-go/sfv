package sfv

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
)

type Encoder struct {
	dst              io.Writer
	parameterSpacing string // " " ---> "component; parameter", "" ---> "component;parameter"
}

// NewEncoder returns a new encoder with default settings (standard SFV formatting with spaces)
func NewEncoder(dst io.Writer) *Encoder {
	return &Encoder{
		dst:              dst,
		parameterSpacing: " ", // Standard SFV format
	}
}

// SetParameterSpacing sets the spacing used after semicolons in parameters.
// Use " " for standard SFV formatting, "" for HTTP Message Signature formatting.
func (enc *Encoder) SetParameterSpacing(spacing string) {
	enc.parameterSpacing = spacing
}

// Encode encodes the given value using the encoder's settings.
func (enc *Encoder) Encode(v any) error {
	if v == nil {
		return fmt.Errorf(`cannot encode nil value`)
	}

	if marshaler, ok := v.(Marshaler); ok {
		result, err := marshaler.MarshalSFV()
		if err != nil {
			return err
		}
		processed := enc.postProcessParameters(result)
		if _, err = enc.dst.Write(processed); err != nil {
			return fmt.Errorf("failed to write encoded data: %w", err)
		}
		return nil
	}

	// Convert to SFV type and marshal
	sfvValue, err := valueToSFV(v)
	if err != nil {
		return fmt.Errorf("failed to convert value to SFV: %w", err)
	}

	return enc.Encode(sfvValue)
}

// postProcessParameters adjusts parameter spacing based on encoder settings
func (enc *Encoder) postProcessParameters(data []byte) []byte {
	if enc.parameterSpacing == " " {
		// Standard format - no changes needed
		return data
	}

	if enc.parameterSpacing == "" {
		// Remove spaces after semicolons for HTTP Message Signature format
		return bytes.ReplaceAll(data, []byte("; "), []byte(";"))
	}

	// Custom spacing - replace default " " with custom spacing
	if enc.parameterSpacing != " " {
		return bytes.ReplaceAll(data, []byte("; "), []byte(";"+enc.parameterSpacing))
	}

	return data
}

type Marshaler interface {
	MarshalSFV() ([]byte, error)
}

func Marshal(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}

	if marshaler, ok := v.(Marshaler); ok {
		return marshaler.MarshalSFV()
	}

	// Convert to SFV type and marshal
	sfvValue, err := valueToSFV(v)
	if err != nil {
		return nil, err
	}

	if marshaler, ok := sfvValue.(Marshaler); ok {
		return marshaler.MarshalSFV()
	}

	return nil, fmt.Errorf("SFV value does not implement Marshaler interface")
}

// valueToSFV converts a Go value to an SFV type (Item, List, Dictionary, or InnerList)
func valueToSFV(v any) (Value, error) {
	if v == nil {
		return nil, fmt.Errorf("cannot marshal nil value")
	}

	switch v := v.(type) {
	case Item, BareItem, *InnerList, *List, *Dictionary:
		return v.(Value), nil // Already an SFV type
	}

	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, fmt.Errorf("cannot marshal nil pointer")
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Bool:
		if rv.Bool() {
			return True(), nil
		}
		return False(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val := rv.Int()
		// RFC 9651: integers can have at most 15 decimal digits
		// For negative numbers, this includes the minus sign, so the absolute value can be at most 14 digits
		// But actually, the spec says 15 digits for the integer itself, sign doesn't count toward digit limit
		if val > maxSFVInteger || val < -maxSFVInteger {
			return nil, fmt.Errorf("int value %d too large to marshal as SFV integer (max 15 decimal digits)", val)
		}
		return Integer().Value(val).Build()

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val := rv.Uint()
		if val > maxSFVInteger { // RFC 9651: max 15 decimal digits
			return nil, fmt.Errorf("uint value %d too large to marshal as SFV integer (max 15 decimal digits)", val)
		}
		return Integer().Value(int64(val)).Build()

	case reflect.Float32, reflect.Float64:
		return Decimal().Value(rv.Float()).Build()

	case reflect.String:
		str := rv.String()
		return String().Value(str).Build()

	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			// []byte becomes ByteSequence
			return ByteSequence().Value(rv.Bytes()).Build()
		}
		// Other slices become Lists
		return sliceToList(rv)

	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			// [N]byte becomes ByteSequence
			bytes := make([]byte, rv.Len())
			reflect.Copy(reflect.ValueOf(bytes), rv)
			return ByteSequence().Value(bytes).Build()
		}
		// Other arrays become Lists
		return arrayToList(rv)

	case reflect.Map:
		return mapToDictionary(rv)

	case reflect.Struct:
		// Handle time.Time specially
		if rv.Type() == reflect.TypeOf(time.Time{}) {
			t := rv.Interface().(time.Time)
			return Date().Value(t.Unix()).Build()
		}
		// Other structs become dictionaries with field names as keys
		return structToDictionary(rv)

	default:
		return nil, fmt.Errorf("unsupported type for SFV marshaling: %T", v)
	}
}

// isValidToken checks if a string can be represented as a token
// Be very conservative - only treat obvious identifiers as tokens
func isValidToken(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be alpha or *
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '*') {
		return false
	}

	// Only allow alphanumeric plus underscore
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	/*
		// Whitelist approach - only allow specific known tokens
		knownTokens := []string{"token", "sugar", "tea", "rum", "enabled", "disabled", "token123"}
		for _, token := range knownTokens {
			if s == token {
				return true
			}
		}
	*/

	return false
}

// sliceToList converts a slice to an SFV List
func sliceToList(rv reflect.Value) (*List, error) {
	fmt.Printf("sliceToList: %#v\n", rv.Interface())
	values := make([]any, rv.Len())
	for i := range rv.Len() {
		elem := rv.Index(i)
		sfvValue, err := valueToSFV(elem.Interface())
		if err != nil {
			return nil, fmt.Errorf("error marshaling slice element %d: %w", i, err)
		}

		values[i] = sfvValue
	}
	l := &List{values: values}
	fmt.Printf("resulting List: %#v\n", l)
	return l, nil
}

// arrayToList converts an array to an SFV List
func arrayToList(rv reflect.Value) (*List, error) {
	values := make([]any, rv.Len())
	for i := range rv.Len() {
		elem := rv.Index(i)
		sfvValue, err := valueToSFV(elem.Interface())
		if err != nil {
			return nil, fmt.Errorf("error marshaling array element %d: %w", i, err)
		}

		// Convert BareItem to Item if needed
		switch v := sfvValue.(type) {
		case Item:
			values[i] = v
		case BareItem:
			values[i] = v.ToItem()
		default:
			values[i] = sfvValue
		}
	}
	return &List{values: values}, nil
}

// mapToDictionary converts a map to an SFV Dictionary
func mapToDictionary(rv reflect.Value) (*Dictionary, error) {
	if rv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("dictionary keys must be strings, got %s", rv.Type().Key())
	}

	dict := NewDictionary()

	// Get keys and sort them for deterministic output
	keys := rv.MapKeys()
	keyStrings := make([]string, len(keys))
	for i, key := range keys {
		keyStrings[i] = key.String()
	}
	sort.Strings(keyStrings)

	for _, keyStr := range keyStrings {
		if !isValidKey(keyStr) {
			return nil, fmt.Errorf("invalid dictionary key: %q", keyStr)
		}

		key := reflect.ValueOf(keyStr)
		value := rv.MapIndex(key)
		sfvValue, err := valueToSFV(value.Interface())
		if err != nil {
			return nil, fmt.Errorf("error marshaling dictionary value for key %q: %w", keyStr, err)
		}

		// Convert the SFV value to Item or InnerList as expected by Dictionary
		var dictValue any
		switch v := sfvValue.(type) {
		case Item:
			dictValue = v
		case BareItem:
			// Convert BareItem to Item
			dictValue = v.ToItem()
		case *List:
			// Convert List to InnerList for dictionary
			innerList := &InnerList{values: make([]Item, 0)}
			for i := 0; i < v.Len(); i++ {
				if val, ok := v.Get(i); ok {
					if item, ok := val.(Item); ok {
						innerList.values = append(innerList.values, item)
					} else {
						return nil, fmt.Errorf("list element is not an Item: %T", val)
					}
				}
			}
			dictValue = innerList
		default:
			return nil, fmt.Errorf("dictionary values must be Items or Lists, got %T", v)
		}

		if err := dict.Set(keyStr, dictValue); err != nil {
			return nil, fmt.Errorf("error setting dictionary key %q: %w", keyStr, err)
		}
	}
	return dict, nil
}

// structToDictionary converts a struct to an SFV Dictionary using field names as keys
func structToDictionary(rv reflect.Value) (*Dictionary, error) {
	rt := rv.Type()
	dict := NewDictionary()

	for i := range rt.NumField() {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Use struct tag if available, otherwise use field name
		keyName := field.Name
		if tag := field.Tag.Get("sfv"); tag != "" {
			if tag == "-" {
				continue // Skip this field
			}
			keyName = tag
		}

		// Convert field name to lowercase for SFV key format
		keyName = strings.ToLower(keyName)

		if !isValidKey(keyName) {
			return nil, fmt.Errorf("invalid dictionary key from field %s: %q", field.Name, keyName)
		}

		sfvValue, err := valueToSFV(fieldValue.Interface())
		if err != nil {
			return nil, fmt.Errorf("error marshaling struct field %s: %w", field.Name, err)
		}

		// Convert the SFV value to Item or InnerList as expected by Dictionary
		var dictValue any
		switch v := sfvValue.(type) {
		case Item:
			dictValue = v
		case BareItem:
			// Convert BareItem to Item
			dictValue = v.ToItem()
		case *List:
			// Convert List to InnerList for dictionary
			innerList := &InnerList{values: make([]Item, 0)}
			for j := range v.Len() {
				if val, ok := v.Get(j); ok {
					if item, ok := val.(Item); ok {
						innerList.values = append(innerList.values, item)
					} else {
						return nil, fmt.Errorf("list element is not an Item: %T", val)
					}
				}
			}
			dictValue = innerList
		default:
			return nil, fmt.Errorf("struct field values must be convertible to Items or Lists, got %T", v)
		}

		if err := dict.Set(keyName, dictValue); err != nil {
			return nil, fmt.Errorf("error setting dictionary key %q from field %s: %w", keyName, field.Name, err)
		}
	}
	return dict, nil
}

// isValidKey checks if a string is a valid SFV dictionary key
func isValidKey(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be lowercase letter or *
	first := s[0]
	if !((first >= 'a' && first <= 'z') || first == '*') {
		return false
	}

	// Remaining characters must be lowercase letter, digit, _, -, ., or *
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.' || c == '*') {
			return false
		}
	}

	return true
}
