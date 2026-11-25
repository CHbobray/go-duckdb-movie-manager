package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"io"
	"log"
	"os"
	"strings"
	"testing"
)

func TestPositiveInt(t *testing.T) {
	for _, s := range []string{"", "abc", "0", "-1", "1.5"} {
		if _, err := positiveInt(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	if n, err := positiveInt(" 31 "); err != nil || n != 31 {
		t.Fatal(n, err)
	}
}

func TestGenreQuery(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range strings.Split(string(schema), ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err = db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{`INSERT INTO movies VALUES (1,'One',2000,8),(2,'Two',2001,6)`, `INSERT INTO genres VALUES (1,'Drama'),(2,'Drama')`} {
		if _, err = db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	var genre string
	var count int
	var avg float64
	if err = db.QueryRow(genreQuery, 2).Scan(&genre, &count, &avg); err != nil {
		t.Fatal(err)
	}
	if genre != "Drama" || count != 2 || avg != 7 {
		t.Fatal(genre, count, avg)
	}
}

// Each test gets its own in-memory database, leaving movies.duckdb untouched.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range strings.Split(string(schema), ";") {
		if strings.TrimSpace(stmt) != "" {
			if _, err := db.Exec(stmt); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, stmt := range []string{
		`INSERT INTO movies VALUES (1,'Spider-Man',2002,7.4),(2,'Spider-Man 2',2004,7.9)`,
		`INSERT INTO genres VALUES (1,'Action'),(2,'Action')`,
		`INSERT INTO actors VALUES (1,'Test','Actor','M')`,
		`INSERT INTO roles VALUES (1,1,1,'Hero'),(2,1,2,NULL)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// Capture actual menu output and logs without changing production code.
// Tests using this helper must not run in parallel.
func menuOutput(t *testing.T, db *sql.DB, input string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "output-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	oldOut := os.Stdout
	oldLog := log.Writer()
	var logs bytes.Buffer
	os.Stdout = f
	log.SetOutput(&logs)
	defer func() { os.Stdout = oldOut; log.SetOutput(oldLog) }()
	run(db, bufio.NewScanner(strings.NewReader(input)))
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(out) + logs.String()
}

func requireOutput(t *testing.T, output string, expected ...string) {
	t.Helper()
	for _, want := range expected {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in output:\n%s", want, output)
		}
	}
}

func TestMenuQueries(t *testing.T) {
	db := testDB(t)
	out := menuOutput(t, db, "1\n2\nspider-man\n3\n1\n3\n2\n3\n999\n5\n0\n")
	requireOutput(t, out, "average_rank", "Spider-Man 2", "Spider-Man | 2002", "Test | Actor | Hero", "(not recorded)", "No matching records.", "personal_rating", "Rows: 2")
}

func TestCollectionLifecycle(t *testing.T) {
	db := testDB(t)
	out := menuOutput(t, db, "4\n1\nLiving Room\n8.5\n5\n4\n1\nHome Theater\n9\n5\n4\n2\nShelf\n\n5\n0\n")
	requireOutput(t, out, "Collection saved.", "Living Room | 8.5", "Home Theater | 9", "Shelf | (not rated)")
	var count int
	var location string
	var rating float64
	if err := db.QueryRow("SELECT COUNT(*) FROM collection WHERE movie_id=1").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one updated record, got %d", count)
	}
	if err := db.QueryRow("SELECT location, personal_rating FROM collection WHERE movie_id=1").Scan(&location, &rating); err != nil {
		t.Fatal(err)
	}
	if location != "Home Theater" || rating != 9 {
		t.Fatalf("unexpected update: %s %v", location, rating)
	}
	var unrated sql.NullFloat64
	if err := db.QueryRow("SELECT personal_rating FROM collection WHERE movie_id=2").Scan(&unrated); err != nil || unrated.Valid {
		t.Fatalf("expected NULL rating: %v %v", unrated, err)
	}
}

func TestMenuInvalidInput(t *testing.T) {
	db := testDB(t)
	out := menuOutput(t, db, "9\n2\n \n3\n-1\n4\nabc\n4\n1\n \n4\n1\nShelf\n11\n4\n1\nShelf\n-1\n4\n1\nShelf\nabc\n4\n1\nShelf\nNaN\n4\n99999999\nShelf\n8.5\n0\n")
	requireOutput(t, out, "Choose a number from 0 to 5.", "Enter a movie title.", "enter a positive whole number", "Location cannot be empty.", "Rating must be between 0 and 10.", "Operation failed:")
	if strings.Count(out, "Rating must be between 0 and 10.") != 4 {
		t.Error("expected four invalid rating messages")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM collection").Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid inputs changed collection: %d %v", count, err)
	}
}

func TestMenuEndOfInput(t *testing.T) {
	db := testDB(t)
	for _, input := range []string{"", "2\n", "3\n", "4\n1\n", "4\n1\nShelf\n"} {
		t.Run(input, func(t *testing.T) { requireOutput(t, menuOutput(t, db, input), "Choice:") })
	}
}

func TestPrintQueryError(t *testing.T) {
	db := testDB(t)
	if err := printQuery(db, "SELECT * FROM missing_table"); err == nil {
		t.Fatal("expected SQL error")
	}
}
