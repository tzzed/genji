// Package document defines types to manipulate and compare documents and values.
package document

import (
	"errors"
	"strconv"
	"strings"
)

// ErrFieldNotFound must be returned by Document implementations, when calling the GetByField method and
// the field wasn't found in the document.
var ErrFieldNotFound = errors.New("field not found")

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

// Length returns the length of a document.
func Length(d Document) (int, error) {
	if fb, ok := d.(*FieldBuffer); ok {
		return fb.Len(), nil
	}

	var len int
	err := d.Iterate(func(_ string, _ Value) error {
		len++
		return nil
	})
	return len, err
}

// FieldBuffer stores a group of fields in memory. It implements the Document interface.
type FieldBuffer struct {
	fields []fieldValue
	key    []byte
}

// NewFieldBuffer creates a FieldBuffer.
func NewFieldBuffer() *FieldBuffer {
	return new(FieldBuffer)
}

type fieldValue struct {
	Field string
	Value Value
}

// Add a field to the buffer.
func (fb *FieldBuffer) Add(field string, v Value) *FieldBuffer {
	fb.fields = append(fb.fields, fieldValue{field, v})
	return fb
}

// ScanDocument copies all the fields of d to the buffer.
func (fb *FieldBuffer) ScanDocument(d Document) error {
	if k, ok := d.(Keyer); ok {
		fb.key = k.Key()
	}

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

// Set replaces a field if it already exists or creates one if not.
func (fb *FieldBuffer) Set(f string, v Value) {
	for i := range fb.fields {
		if fb.fields[i].Field == f {
			fb.fields[i].Value = v
			return
		}
	}

	fb.Add(f, v)
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

// Key of the document if any.
func (fb *FieldBuffer) Key() []byte {
	return fb.key
}

// A ValuePath represents the path to a particular value within a document.
type ValuePath []ValuePathFragment

// ValuePathFragment is a fragment of a path representing either a field name or
// the index of an array.
type ValuePathFragment struct {
	FieldName  string
	ArrayIndex int
}

// String representation of all the fragments of the path.
// It implements the Stringer interface.
func (p ValuePath) String() string {
	var b strings.Builder

	for i := range p {
		if p[i].FieldName != "" {
			if i != 0 {
				b.WriteRune('.')
			}
			b.WriteString(p[i].FieldName)
		} else {
			b.WriteString("[" + strconv.Itoa(p[i].ArrayIndex) + "]")
		}
	}
	return b.String()
}

// GetValue from a document.
func (p ValuePath) GetValue(d Document) (Value, error) {
	return p.getValueFromDocument(d)
}

func (p ValuePath) getValueFromDocument(d Document) (Value, error) {
	if len(p) == 0 {
		return Value{}, ErrFieldNotFound
	}
	if p[0].FieldName == "" {
		return Value{}, ErrFieldNotFound
	}

	v, err := d.GetByField(p[0].FieldName)
	if err != nil {
		return Value{}, err
	}

	if len(p) == 1 {
		return v, nil
	}

	return p[1:].getValueFromValue(v)
}

func (p ValuePath) getValueFromArray(a Array) (Value, error) {
	if len(p) == 0 {
		return Value{}, ErrFieldNotFound
	}
	if p[0].FieldName != "" {
		return Value{}, ErrFieldNotFound
	}

	v, err := a.GetByIndex(p[0].ArrayIndex)
	if err != nil {
		if err == ErrValueNotFound {
			return Value{}, ErrFieldNotFound
		}

		return Value{}, err
	}

	if len(p) == 1 {
		return v, nil
	}

	return p[1:].getValueFromValue(v)
}

func (p ValuePath) getValueFromValue(v Value) (Value, error) {
	switch v.Type {
	case DocumentValue:
		return p.getValueFromDocument(v.V.(Document))
	case ArrayValue:
		return p.getValueFromArray(v.V.(Array))
	}

	return Value{}, ErrFieldNotFound
}
