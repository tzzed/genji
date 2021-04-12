package dbutil

import (
	"context"
	"fmt"
	"io"
	"strings"
	
	"github.com/genjidb/genji"
	"github.com/genjidb/genji/document"
	"go.uber.org/multierr"
)

// Dump takes a database and dumps its content as SQL queries in the given writer.
// If tables is provided, only selected tables will be outputted.
func Dump(ctx context.Context, db *genji.DB, w io.Writer, tables ...string) error {
	tx, err := db.Begin(false)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	if _, err = fmt.Fprintln(w, "BEGIN TRANSACTION;"); err != nil {
		return err
	}
	
	query := "SELECT table_name FROM __genji_tables"
	if len(tables) > 0 {
		query += " WHERE table_name IN ?"
	}
	
	res, err := tx.Query(query, tables)
	if err != nil {
		_, er := fmt.Fprintln(w, "ROLLBACK;")
		return multierr.Append(err, er)
	}
	defer res.Close()
	
	i := 0
	err = res.Iterate(func(d document.Document) error {
		// Blank separation between tables.
		if i > 0 {
			if _, err := fmt.Fprintln(w, ""); err != nil {
				return err
			}
		}
		i++
		
		// Get table name.
		var tableName string
		if err := document.Scan(d, &tableName); err != nil {
			return err
		}
		
		return dumpTable(tx, w, tableName)
	})
	if err != nil {
		_, er := fmt.Fprintln(w, "ROLLBACK;")
		return multierr.Append(err, er)
	}
	
	_, err = fmt.Fprintln(w, "COMMIT;")
	return err
}

// dumpTable displays the content of the given table as SQL statements.
func dumpTable(tx *genji.Tx, w io.Writer, tableName string) error {
	// Dump schema first.
	if err := dumpSchema(tx, w, tableName); err != nil {
		return err
	}
	
	q := fmt.Sprintf("SELECT * FROM %s", tableName)
	res, err := tx.Query(q)
	if err != nil {
		return err
	}
	defer res.Close()
	
	// Inserts statements.
	insert := fmt.Sprintf("INSERT INTO %s VALUES", tableName)
	return res.Iterate(func(d document.Document) error {
		data, err := document.MarshalJSON(d)
		if err != nil {
			return err
		}
		
		if _, err := fmt.Fprintf(w, "%s %s;\n", insert, string(data)); err != nil {
			return err
		}
		
		return nil
	})
}

// DumpSchema takes a database and dumps its schema as SQL queries in the given writer.
// If tables are provided, only selected tables will be outputted.
func DumpSchema(ctx context.Context, db *genji.DB, w io.Writer, tables ...string) error {
	tx, err := db.Begin(false)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	query := "SELECT table_name FROM __genji_tables"
	if len(tables) > 0 {
		query += " WHERE table_name IN ?"
	}
	
	res, err := tx.Query(query, tables)
	if err != nil {
		return err
	}
	defer res.Close()
	
	i := 0
	return res.Iterate(func(d document.Document) error {
		// Blank separation between tables.
		if i > 0 {
			if _, err := fmt.Fprintln(w, ""); err != nil {
				return err
			}
		}
		i++
		
		// Get table name.
		var tableName string
		if err := document.Scan(d, &tableName); err != nil {
			return err
		}
		
		return dumpSchema(tx, w, tableName)
	})
}

// dumpSchema displays the schema of the given table as SQL statements.
func dumpSchema(tx *genji.Tx, w io.Writer, tableName string) error {
	t, err := tx.GetTable(tableName)
	if err != nil {
		return err
	}
	
	_, err = fmt.Fprintf(w, "CREATE TABLE %s", tableName)
	if err != nil {
		return err
	}
	
	ti := t.Info()
	
	fcs := ti.FieldConstraints
	// Fields constraints should be displayed between parenthesis.
	if len(fcs) > 0 {
		_, err = fmt.Fprintln(w, " (")
		if err != nil {
			return err
		}
	}
	
	for i, fc := range fcs {
		// Don't display the last comma.
		if i > 0 {
			_, err = fmt.Fprintln(w, ",")
			if err != nil {
				return err
			}
		}
		
		// Construct the fields constraints
		if _, err := fmt.Fprintf(w, " %s %s", fcs[i].Path.String(), strings.ToUpper(fcs[i].Type.String())); err != nil {
			return err
		}
		
		f := ""
		if fc.IsPrimaryKey {
			f += " PRIMARY KEY"
		}
		
		if fc.IsNotNull {
			f += " NOT NULL"
		}
		
		if fc.HasDefaultValue() {
			if _, err := fmt.Fprintf(w, "%s DEFAULT %s", f, fc.DefaultValue.String()); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, f); err != nil {
				return err
			}
		}
	}
	
	// Fields constraints close parenthesis.
	if len(fcs) > 0 {
		if _, err := fmt.Fprintln(w, "\n);"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, ";"); err != nil {
			return err
		}
	}
	
	// Indexes statements.
	indexes := t.Indexes()
	
	for _, index := range indexes {
		u := ""
		if index.Info.Unique {
			u = " UNIQUE"
		}
		
		_, err = fmt.Fprintf(w, "CREATE%s INDEX %s ON %s (%s);\n", u, index.Info.IndexName, index.Info.TableName,
			index.Info.Path)
		if err != nil {
			return err
		}
	}
	
	return nil
}
