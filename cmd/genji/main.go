package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/genjidb/genji/cmd/genji/shell"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "Genji"
	app.Usage = "Shell for the Genji database"
	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:  "bolt",
			Usage: "use bolt engine",
		},
		&cli.BoolFlag{
			Name:  "badger",
			Usage: "use badger engine",
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:      "insert",
			Usage:     "Insert documents from arguments or standard input",
			UsageText: "genji insert [options] [json...]",
			Description: `
The insert command inserts documents into an existing table.

Insert can take JSON documents as separate arguments:

$ genji insert --db my.db -t foo '{"a": 1}' '{"a": 2}'

It is also possible to pass an array of objects:

$ genji insert --db my.db -t foo '[{"a": 1}, {"a": 2}]'

Also you can use -a flag to create database automatically.
This example will create BoltDB-based database with name 'data_${current unix timestamp}.db'
It can be combined with --db to select an existing database but automatically create the table.

$ genji insert -a -e bolt '[{"a": 1}, {"a": 2}]'

Insert can also insert a stream of objects or an array of objects from standard input:

$ echo '{"a": 1} {"a": 2}' | genji insert --db my.db -t foo
$ echo '[{"a": 1},{"a": 2}]' | genji insert --db my.db -t foo
$ curl https://api.github.com/repos/genjidb/genji/issues | genji insert --db my.db -t foo`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "engine",
					Aliases: []string{"e"},
					Usage:   "name of the engine to use, options are 'bolt' or 'badger'",
					Value:   "bolt",
				},
				&cli.StringFlag{
					Name:     "db",
					Usage:    "path of the database file",
					Required: false,
				},
				&cli.StringFlag{
					Name:     "table",
					Aliases:  []string{"t"},
					Usage:    "name of the table, it must already exist",
					Required: false,
				},
				&cli.BoolFlag{
					Name:     "auto",
					Aliases:  []string{"a"},
					Usage:    `automatically creates a database and a table whose name is equal to "data_" followed by the current unix timestamp.`,
					Required: false,
					Value:    false,
				},
			},
			Action: func(c *cli.Context) error {
				dbPath := c.String("db")
				table := c.String("table")
				engine := c.String("engine")
				args := c.Args().Slice()

				return runInsertCommand(c.Context, engine, dbPath, table, c.Bool("auto"), args)
			},
		},
		{
			Name:  "version",
			Usage: "Shows Genji and Genji CLI version",
			Action: func(c *cli.Context) error {
				var cliVersion, genjiVersion string
				info, ok := debug.ReadBuildInfo()

				if !ok {
					fmt.Println(`version not available in GOPATH mode; use "go get" with Go modules enabled`)
					return nil
				}

				cliVersion = info.Main.Version
				for _, mod := range info.Deps {
					if mod.Path != "github.com/genjidb/genji" {
						continue
					}
					genjiVersion = mod.Version
					break
				}
				fmt.Printf("Genji %v\nGenji CLI %v\n", genjiVersion, cliVersion)
				return nil
			},
		},
		{
			Name:      "dump",
			Usage:     "Dump a database or a list of tables as a text file.",
			UsageText: `genji dump [options] dbpath`,
			Description: ` The dump command can dump a database as a text file.

  By default, the content of the database is sent to the standard output:

  $ genji dump my.db
  CREATE TABLE foo;
  ...

  It is possible to specify a list of tables to output:

  $ genji dump -t foo -f bar my.db

  The dump command can also write directly into a file:

  $ genji dump -f dump.sql my.db`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "file",
					Aliases: []string{"f"},
					Usage:   "name of the file to output to. Defaults to STDOUT.",
				},
				&cli.StringSliceFlag{
					Name:    "table",
					Aliases: []string{"t"},
					Usage:   "name of the table, it must already exist. Default to all tables.",
				},
				&cli.StringFlag{
					Name:    "engine",
					Aliases: []string{"e"},
					Usage:   "name of the engine to use, options are 'bolt' or 'badger'",
					Value:   "bolt",
				},
			},
			Action: func(c *cli.Context) error {
				table := c.StringSlice("table")
				f := c.String("file")
				engine := c.String("engine")
				dbPath := c.Args().First()
				if dbPath == "" {
					return errors.New("expected db path, got empty")
				}

				var w io.Writer
				w = os.Stdout

				if f != "" {
					file, err := os.Create(f)
					if err != nil {
						return err
					}
					defer file.Close()

					// file as io.writer for the RunDumpCmd function.
					w = file
				}

				return executeDump(c.Context, w, table, engine, dbPath)
			},
		},
		{
			Name:      "restore",
			Usage:     "Restore a database from a file created by genji dump",
			UsageText: `genji restore dumpFile dbPath`,
			Description: `The restore command can restore a database from a text file.

		$ genji restore -f dump.sql -e bolt my.db`,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "engine",
					Aliases: []string{"e"},
					Usage:   "name of the engine to use, options are 'bolt' or 'badger'",
					Value:   "bolt",
				},
			},
			Action: func(c *cli.Context) error {
				
				engine := c.String("engine")
				f := c.Args().First()
				dbPath := c.Args().Get(c.Args().Len()-1)
				if dbPath == "" {
					return errors.New("expected database path")
				}

				var r io.Reader
				if f != "" {
					file, err := os.Open(f)
					if err != nil {
						return err
					}
					defer file.Close()

					r = file
				}

				return executeRestore(c.Context, r, engine, dbPath)
			},
		},
	}

	// Root command
	app.Action = func(c *cli.Context) error {
		useBolt := c.Bool("bolt")
		useBadger := c.Bool("badger")
		if useBolt && useBadger {
			return cli.NewExitError("cannot use bolt and badger options at the same time", 2)
		}

		dbpath := c.Args().First()

		if (useBolt || useBadger) && dbpath == "" {
			return cli.NewExitError("db path required when using bolt or badger", 2)
		}

		engine := "memory"

		if useBolt || dbpath != "" {
			engine = "bolt"
		}

		if useBadger {
			engine = "badger"
		}

		return shell.Run(c.Context, &shell.Options{
			Engine: engine,
			DBPath: dbpath,
		})
	}

	err := app.Run(os.Args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "error: %v\n", err)
		os.Exit(2)
	}
}
