package msgpack

import (
	"bytes"
	"fmt"
	"io"

	"github.com/genjidb/genji/document"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/vmihailenco/msgpack/v5/codes"
)

// EncodeDocument takes a document and encodes it
// in MessagePack.
func EncodeDocument(d document.Document) ([]byte, error) {
	if ec, ok := d.(EncodedDocument); ok {
		return ec, nil
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	defer enc.Close()

	err := enc.EncodeDocument(d)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DecodeDocument takes a byte slice and returns a lazily decoded document.
// If buf is malformed, an error will be returned when calling one of the document method.
func DecodeDocument(buf []byte) document.Document {
	return EncodedDocument(buf)
}

// EncodeValue encodes any value to MessagePack.
func EncodeValue(w io.Writer, v document.Value) error {
	enc := NewEncoder(w)
	defer enc.Close()

	return enc.EncodeValue(v)
}

// An EncodedDocument implements the document.Document
// interface on top of an encoded representation of a
// document.
// It is useful for avoiding decoding the entire document when
// only a few fields are needed.
type EncodedDocument []byte

// bytesLen determines the size of the next string in the decoder
// based on c.
// It is originally copied from https://github.com/vmihailenco/msgpack/blob/e7759683b74a27e455669b525427cfd9aec0790e/decode_string.go#L10:19
// then adapted to our needs.
func bytesLen(c codes.Code, dec *msgpack.Decoder) (int, error) {
	if c == codes.Nil {
		return -1, nil
	}

	if codes.IsFixedString(c) {
		return int(c & codes.FixedStrMask), nil
	}

	switch c {
	case codes.Str8, codes.Bin8:
		var b [1]byte
		err := dec.ReadFull(b[:])
		return int(b[0]), err
	case codes.Str16, codes.Bin16:
		var b [2]byte
		err := dec.ReadFull(b[:])
		return int((uint16(b[0]) << 8) | uint16(b[1])), err
	case codes.Str32, codes.Bin32:
		var b [4]byte
		err := dec.ReadFull(b[:])
		n := (uint32(b[0]) << 24) |
			(uint32(b[1]) << 16) |
			(uint32(b[2]) << 8) |
			uint32(b[3])
		return int(n), err
	}

	return 0, fmt.Errorf("msgpack: invalid code=%x decoding bytes length", c)
}

// GetByField decodes the selected field from the buffer.
func (e EncodedDocument) GetByField(field string) (v document.Value, err error) {
	dec := NewDecoder(bytes.NewReader(e))
	defer dec.Close()

	l, err := dec.dec.DecodeMapLen()
	if err != nil {
		return
	}

	bf := []byte(field)

	buf := make([]byte, 32)

	var c codes.Code
	var n int
	for i := 0; i < l; i++ {
		// this loop does basically two things:
		// - decode the field name
		// - decode the value
		// We don't use dec.dec.DecodeString() here
		// because it allocates a new string at every call
		// which is not memory efficient.
		// Since we only want to compare the field name with
		// the one received in parameter, we will decode
		// the field name ourselves and reuse the buffer
		// everytime.

		// get the type code from the decoder.
		// PeekCode doesn't move the cursor
		c, err = dec.dec.PeekCode()
		if err != nil {
			return
		}

		// Move the cursor by one to skip the type code
		err = dec.dec.ReadFull(buf[:1])
		if err != nil {
			return
		}

		// determine the string length
		n, err = bytesLen(c, dec.dec)
		if err != nil {
			return
		}

		// ensure the buffer is big enough to hold the string
		if len(buf) < n {
			buf = make([]byte, n)
		}

		// copy the field name into the buffer
		err = dec.dec.ReadFull(buf[:n])
		if err != nil {
			return
		}

		// if the field name is the one we are
		// looking for, decode the next value
		if bytes.Equal(buf[:n], bf) {
			return dec.DecodeValue()
		}

		// if not, we skip the next value
		err = dec.dec.Skip()
		if err != nil {
			return
		}
	}

	err = document.ErrFieldNotFound
	return
}

// Iterate decodes each fields one by one and passes them to fn
// until the end of the document or until fn returns an error.
func (e EncodedDocument) Iterate(fn func(field string, value document.Value) error) error {
	dec := NewDecoder(bytes.NewReader(e))
	defer dec.Close()

	l, err := dec.dec.DecodeMapLen()
	if err != nil {
		return err
	}

	for i := 0; i < l; i++ {
		f, err := dec.dec.DecodeString()
		if err != nil {
			return err
		}

		v, err := dec.DecodeValue()
		if err != nil {
			return err
		}

		err = fn(f, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (e EncodedDocument) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	err := document.ToJSON(&buf, e)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// An EncodedArray implements the document.Array interface on top of an
// encoded representation of an array.
// It is useful for avoiding decoding the entire array when
// only a few values are needed.
type EncodedArray []byte

// Iterate goes through all the values of the array and calls the
// given function by passing each one of them.
// If the given function returns an error, the iteration stops.
func (e EncodedArray) Iterate(fn func(i int, value document.Value) error) error {
	dec := NewDecoder(bytes.NewReader(e))
	defer dec.Close()

	l, err := dec.dec.DecodeArrayLen()
	if err != nil {
		return err
	}

	for i := 0; i < l; i++ {
		v, err := dec.DecodeValue()
		if err != nil {
			return err
		}

		err = fn(i, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetByIndex returns a value by index of the array.
func (e EncodedArray) GetByIndex(idx int) (v document.Value, err error) {
	dec := NewDecoder(bytes.NewReader(e))
	defer dec.Close()

	l, err := dec.dec.DecodeArrayLen()
	if err != nil {
		return
	}

	for i := 0; i < l; i++ {
		if i == idx {
			return dec.DecodeValue()
		}

		err = dec.dec.Skip()
		if err != nil {
			return
		}
	}

	err = document.ErrValueNotFound
	return
}

// MarshalJSON implements the json.Marshaler interface.
func (e EncodedArray) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	err := document.ArrayToJSON(&buf, e)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// EncodeArray encodes a into its binary representation.
func EncodeArray(a document.Array) ([]byte, error) {
	if ea, ok := a.(EncodedArray); ok {
		return ea, nil
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	defer enc.Close()

	err := enc.EncodeArray(a)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DecodeArray takes a byte slice and returns a lazily decoded array.
// If buf is malformed, an error will be returned when calling one of the array method.
func DecodeArray(buf []byte) document.Array {
	return EncodedArray(buf)
}

// DecodeValue takes some encoded data and decodes it to the target type t.
func DecodeValue(data []byte) (document.Value, error) {
	dec := NewDecoder(bytes.NewReader(data))
	defer dec.Close()

	return dec.DecodeValue()
}
