package genji_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/genjidb/genji"
	"github.com/genjidb/genji/database"
	"github.com/genjidb/genji/document"
	"github.com/stretchr/testify/require"
)

func ExampleTx() {
	db, err := genji.Open(":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.Begin(true)
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	err = tx.Exec("CREATE TABLE IF NOT EXISTS user")
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Exec("INSERT INTO user (id, name, age) VALUES (?, ?, ?)", 10, "foo", 15)
	if err != nil {
		log.Fatal(err)
	}

	d, err := tx.QueryDocument("SELECT id, name, age FROM user WHERE name = ?", "foo")
	if err != nil {
		panic(err)
	}

	var u User
	err = document.StructScan(d, &u)
	if err != nil {
		panic(err)
	}

	fmt.Println(u)

	var id uint64
	var name string
	var age uint8

	err = document.Scan(d, &id, &name, &age)
	if err != nil {
		panic(err)
	}

	fmt.Println(id, name, age)

	err = tx.Commit()
	if err != nil {
		panic(err)
	}

	// Output: {10 foo 15 { }}
	// 10 foo 15
}

func TestQueryDocument(t *testing.T) {
	db, err := genji.Open(":memory:")
	require.NoError(t, err)

	tx, err := db.Begin(true)
	require.NoError(t, err)

	err = tx.Exec(`
			CREATE TABLE test;
			INSERT INTO test (a, b) VALUES (1, 'foo'), (2, 'bar')
		`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	t.Run("Should return the first document", func(t *testing.T) {
		var a int
		var b string

		r, err := db.QueryDocument("SELECT * FROM test")
		err = document.Scan(r, &a, &b)
		require.NoError(t, err)
		require.Equal(t, 1, a)
		require.Equal(t, "foo", b)

		tx, err := db.Begin(false)
		require.NoError(t, err)
		defer tx.Rollback()

		r, err = tx.QueryDocument("SELECT * FROM test")
		require.NoError(t, err)
		err = document.Scan(r, &a, &b)
		require.NoError(t, err)
		require.Equal(t, 1, a)
		require.Equal(t, "foo", b)
	})

	t.Run("Should return an error if no document", func(t *testing.T) {
		r, err := db.QueryDocument("SELECT * FROM test WHERE a > 100")
		require.Equal(t, database.ErrDocumentNotFound, err)
		require.Nil(t, r)

		tx, err := db.Begin(false)
		require.NoError(t, err)
		defer tx.Rollback()
		r, err = tx.QueryDocument("SELECT * FROM test WHERE a > 100")
		require.Equal(t, database.ErrDocumentNotFound, err)
		require.Nil(t, r)
	})
}
