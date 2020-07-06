// Package document defines types to manipulate and compare documents and values.
package document

import (
	"errors"
	"strconv"
	"strings"
)

// ErrFieldNotFound must be returned by Document implementations, when calling the GetByField method and
// the field wasn't found in the document.
var (
	ErrFieldNotFound   = errors.New("field not found")
	ErrShortNotation   = errors.New("Short Notation")
	ErrCreateField     = errors.New("field must be created")
	ErrFieldReplaced   = errors.New("buffer is replaced")
	ErrValueNotSet     = errors.New("value not set")
	ErrIndexOutOfBound = errors.New("index out of bounds")
	ErrNotDocument     = errors.New("type must be  a document")
)

const NEXTFIELD = 1
// A Document represents a group of key value pairs.
type Document interface {
	// Iterate goes through all the fields of the document and calls the given function by passing each one of them.
	// If the given function returns an error, the iteration stops.
	Iterate(fn func(field string, value Value) error) error
	// GetByField returns a value by field name.
	// Must return ErrFieldNotFound if the field doesnt exist.
	GetByField(field string) (Value, error)
}

// A Keyer returns the key identifying documents in their storage.
// This is usually implemented by documents read from storages.
type Keyer interface {
	Key() []byte
}

// FieldBuffer stores a group of fields in memory. It implements the Document interface.
type FieldBuffer struct {
	fields []fieldValue
}

//fieldValue  Document structure field and value pairs.
type fieldValue struct {
	Field string
	Value Value
}

// NewFieldBuffer creates a FieldBuffer.
func NewFieldBuffer() *FieldBuffer {
	return new(FieldBuffer)
}

// NewFieldBufferByCopy creates a FieldBuffer from a DocumentValue.
func NewFieldBufferByCopy(v Value) (FieldBuffer, error) {
	var buf FieldBuffer
	err := buf.Copy(v.V.(Document))
	if err != nil {
		return *NewFieldBuffer(), ErrNotDocument
	}

	return buf, nil
}


// Add a field to the buffer.
func (fb *FieldBuffer) Add(field string, v Value) *FieldBuffer {
	fb.fields = append(fb.fields, fieldValue{field, v})
	return fb
}

// ScanDocument copies all the fields of d to the buffer.
func (fb *FieldBuffer) ScanDocument(d Document) error {
	return d.Iterate(func(f string, v Value) error {
		fb.Add(f, v)
		return nil
	})
}

// GetByField returns a value by field. Returns an error if the field doesn't exists.
func (fb FieldBuffer) GetByField(field string) (Value, error) {
	for _, fv := range fb.fields {
		if fv.Field == field {
			return fv.Value, nil
		}
	}

	return Value{}, ErrFieldNotFound
}

// IsUniqueField is the ValuePath only a field to set <field>
func (path ValuePath) IsUniqueField() bool {
	if len(path) == 1 {
		return true
	}

	return false
}

// SetUniqueFieldOfDocument Add/Replace a value if the request is <Document.Field> or <Array.Index> or field.
func (fb *FieldBuffer) SetUniqueFieldOfDocument(field string, reqValue Value) error {
	_, err := fb.GetByField(field)
	switch err {
	case ErrFieldNotFound:
		fb.Add(field, reqValue)
		return nil
	case nil:
		_ = fb.Replace(field, reqValue)
		return nil
	}

	return err
}

// SetDocument make deep replacement or creation of a field.
func (fb *FieldBuffer) SetDocument(v Value, path ValuePath, reqValue Value) (Value, error) {
	switch v.Type {
	case DocumentValue:
		buf, err := NewFieldBufferByCopy(v)
		if err != nil {
			return v, err
		}

		if path.IsUniqueField() {
			buf.SetUniqueFieldOfDocument(path[0], reqValue)
			return NewDocumentValue(buf), nil
		}

		va, err := buf.GetByField(path[0])
		if err != nil {
			return v, err
		}

		va, err = buf.SetDocument(va, path[NEXTFIELD:], reqValue)
		if err != nil {
			return v, err
		}

		buf.SetUniqueFieldOfDocument(path[0], va)
		return NewDocumentValue(buf), nil
	case ArrayValue:
		buf, err := NewValueBufferByCopy(v)
		if err != nil {
			return v, err
		}

		va, index, err := buf.GetValueFromString(path[0])
		if err != nil {
			return v, err
		}

		if path.IsUniqueField() {
			_ = buf.Replace(index, reqValue)
			return NewArrayValue(buf), nil
		}

		va, err = fb.SetDocument(va, path[NEXTFIELD:], reqValue)
		_ = buf.Replace(index, va)
		return NewArrayValue(buf), nil
	}

	return v, nil
}

// Set replaces a field if it already exists or creates one if not.
func (fb *FieldBuffer) Set(path ValuePath, reqValue Value) error {
	if path.IsUniqueField() {
		return fb.SetUniqueFieldOfDocument(path[0], reqValue)
	}

	for i, field := range fb.fields {
		if path[0] == field.Field {
			v, err := fb.SetDocument(field.Value, path[1:], reqValue)
			if err != nil {
				return err
			}

			fb.fields[i].Value = v
			return nil
		}
	}

	return nil
}

// Iterate goes through all the fields of the document and calls the given function by passing each one of them.
// If the given function returns an error, the iteration stops.
func (fb FieldBuffer) Iterate(fn func(field string, value Value) error) error {
	for _, fv := range fb.fields {
		err := fn(fv.Field, fv.Value)
		if err != nil {
			return err
		}
	}

	return nil
}

// Delete a field from the buffer.
func (fb *FieldBuffer) Delete(field string) error {
	for i := range fb.fields {
		if fb.fields[i].Field == field {
			fb.fields = append(fb.fields[0:i], fb.fields[i+1:]...)
			return nil
		}
	}

	return ErrFieldNotFound
}

// Replace the value of the field by v.
func (fb *FieldBuffer) Replace(field string, v Value) error {

	for i := range fb.fields {
		if fb.fields[i].Field == field {
			fb.fields[i].Value = v
			return nil
		}
	}

	return ErrFieldNotFound
}

// Copy deep copies every value of the document to the buffer.
// If a value is a document or an array, it will be stored as a FieldBuffer or ValueBuffer respectively.
func (fb *FieldBuffer) Copy(d Document) error {
	err := fb.ScanDocument(d)
	if err != nil {
		return err
	}

	for i, f := range fb.fields {
		switch f.Value.Type {
		case DocumentValue:
			var buf FieldBuffer
			err = buf.Copy(f.Value.V.(Document))
			if err != nil {
				return err
			}

			fb.fields[i].Value = NewDocumentValue(&buf)
		case ArrayValue:
			var buf ValueBuffer
			err = buf.Copy(f.Value.V.(Array))
			if err != nil {
				return err
			}
			fb.fields[i].Value = NewArrayValue(&buf)
		}
	}

	return nil
}

// Len of the buffer.
func (fb FieldBuffer) Len() int {
	return len(fb.fields)
}

// Reset the buffer.
func (fb *FieldBuffer) Reset() {
	fb.fields = fb.fields[:0]
}

// A ValuePath represents the path to a particular value within a document.
type ValuePath []string

// NewValuePath takes a string representation of a value path and returns a ValuePath.
// It assumes the separator is a dot.
func NewValuePath(p string) ValuePath {
	return strings.Split(p, ".")
}

// String joins all the chunks of the path using the dot separator.
// It implements the Stringer interface.
func (p ValuePath) String() string {
	return strings.Join(p, ".")
}

// GetValue from a document.
func (p ValuePath) GetValue(d Document) (Value, error) {
	return p.getValueFromDocument(d)
}

func (p ValuePath) getValueFromDocument(d Document) (Value, error) {
	if len(p) == 0 {
		return Value{}, errors.New("empty valuepath")
	}

	v, err := d.GetByField(p[0])
	if err != nil {
		return Value{}, err
	}

	return p.getValueFromValue(v)
}

func (p ValuePath) getValueFromValue(v Value) (Value, error) {
	if len(p) == 1 {
		return v, nil
	}

	switch v.Type {
	case DocumentValue:
		if len(p) == 1 {
			return v, nil
		}

		d, err := v.ConvertToDocument()
		if err != nil {
			return Value{}, err
		}

		return p[1:].getValueFromDocument(d)
	case ArrayValue:

		a, err := v.ConvertToArray()
		if err != nil {
			return Value{}, err
		}

		return p[1:].getValueFromArray(a)
	}

	return Value{}, ErrFieldNotFound
}
