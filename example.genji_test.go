// Code generated by genji.
// DO NOT EDIT!

package genji_test

import (
	"errors"

	"github.com/asdine/genji"
	"github.com/asdine/genji/field"
	"github.com/asdine/genji/query"
	"github.com/asdine/genji/record"
	"github.com/asdine/genji/table"
)

// Field implements the field method of the record.Record interface.
func (u *User) Field(name string) (field.Field, error) {
	switch name {
	case "ID":
		return field.NewInt64("ID", u.ID), nil
	case "Name":
		return field.NewString("Name", u.Name), nil
	case "Age":
		return field.NewUint32("Age", u.Age), nil
	}

	return field.Field{}, errors.New("unknown field")
}

// Iterate through all the fields one by one and pass each of them to the given function.
// It the given function returns an error, the iteration is interrupted.
func (u *User) Iterate(fn func(field.Field) error) error {
	var err error
	var f field.Field

	f, _ = u.Field("ID")
	err = fn(f)
	if err != nil {
		return err
	}

	f, _ = u.Field("Name")
	err = fn(f)
	if err != nil {
		return err
	}

	f, _ = u.Field("Age")
	err = fn(f)
	if err != nil {
		return err
	}

	return nil
}

// ScanRecord extracts fields from record and assigns them to the struct fields.
// It implements the record.Scanner interface.
func (u *User) ScanRecord(rec record.Record) error {
	return rec.Iterate(func(f field.Field) error {
		var err error

		switch f.Name {
		case "ID":
			u.ID, err = field.DecodeInt64(f.Data)
		case "Name":
			u.Name, err = field.DecodeString(f.Data)
		case "Age":
			u.Age, err = field.DecodeUint32(f.Data)
		}
		return err
	})
}

// Pk returns the primary key. It implements the table.Pker interface.
func (u *User) Pk() ([]byte, error) {
	return field.EncodeInt64(u.ID), nil
}

// UserTable manages the User table.
type UserTable struct {
	ID   query.Int64FieldSelector
	Name query.StringFieldSelector
	Age  query.Uint32FieldSelector
}

// NewUserTable creates a UserTable.
func NewUserTable() *UserTable {
	return &UserTable{
		ID:   query.Int64Field("ID"),
		Name: query.StringField("Name"),
		Age:  query.Uint32Field("Age"),
	}
}

// Init initializes the User table by ensuring the table and its index are created.
func (t *UserTable) Init(tx *genji.Tx) error {
	return genji.InitTable(tx, t)
}

// SelectTable implements the query.TableSelector interface. It gets the User table from
// the transaction.
func (t *UserTable) SelectTable(tx *genji.Tx) (*genji.Table, error) {
	return tx.Table(t.TableName())
}

// Insert is a shortcut that gets the User table from the transaction and
// inserts a User into it.
func (t *UserTable) Insert(tx *genji.Tx, x *User) error {
	tb, err := t.SelectTable(tx)
	if err != nil {
		return err
	}

	_, err = tb.Insert(x)
	return err
}

// TableName returns the name of the table.
func (*UserTable) TableName() string {
	return "User"
}

// Indexes returns the list of indexes of the User table.
func (*UserTable) Indexes() []string {
	return []string{
		"Name",
	}
}

// All returns a list of all selectors for User.
func (t *UserTable) All() []query.FieldSelector {
	return []query.FieldSelector{
		t.ID,
		t.Name,
		t.Age,
	}
}

// UserResult can be used to store the result of queries.
// Selected fields must map the User fields.
type UserResult []User

// ScanTable iterates over table.Reader and stores all the records in the slice.
func (u *UserResult) ScanTable(tr table.Reader) error {
	return tr.Iterate(func(_ []byte, r record.Record) error {
		var record User
		err := record.ScanRecord(r)
		if err != nil {
			return err
		}

		*u = append(*u, record)
		return nil
	})
}
