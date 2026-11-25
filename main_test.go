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

// TestPositiveInt checks that only whole numbers above zero are accepted.
func TestPositiveInt(t *testing.T) {
	for _, text := range []string{"", "abc", "0", "-1", "1.5"} {
		if _, err := positiveInt(text); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
	if number, err := positiveInt(" 31 "); err != nil || number != 31 {
		t.Fatal(number, err)
	}
}

// TestParseRating checks the rating limits, blank input and invalid text.
func TestParseRating(t *testing.T) {
	for _, text := range []string{"0", "10", " 8.5 "} {
		if _, err := parseRating(text); err != nil {
			t.Errorf("rejected %q: %v", text, err)
		}
	}
	for _, text := range []string{"-0.1", "10.1", "abc", "NaN", "Inf"} {
		if _, err := parseRating(text); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
	if rating, err := parseRating("  "); rating != nil || err != nil {
		t.Errorf("blank rating should be nil, got %v %v", rating, err)
	}
}

// emptyDB creates an isolated in-memory database with the project schema,
// leaving movies.duckdb untouched.
func emptyDB(tb testing.TB) *sql.DB {
	tb.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		tb.Fatal(err)
	}
	// One connection keeps every query on the same in-memory database.
	db.SetMaxOpenConns(1)
	tb.Cleanup(func() { db.Close() })
	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		tb.Fatal(err)
	}
	for _, statement := range strings.Split(string(schema), ";") {
		if strings.TrimSpace(statement) != "" {
			execAll(tb, db, statement)
		}
	}
	return db
}

// execAll runs setup statements and stops the test if any fail.
func execAll(tb testing.TB, db *sql.DB, statements ...string) {
	tb.Helper()
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			tb.Fatal(err)
		}
	}
}

// testDB returns a small synthetic database with two movies, one actor and
// one role with a missing description.
func testDB(tb testing.TB) *sql.DB {
	tb.Helper()
	db := emptyDB(tb)
	execAll(tb, db,
		`INSERT INTO movies VALUES (1,'Spider-Man',2002,7.4),(2,'Spider-Man 2',2004,7.9)`,
		`INSERT INTO genres VALUES (1,'Action'),(2,'Action')`,
		`INSERT INTO actors VALUES (1,'Test','Actor','M')`,
		`INSERT INTO roles VALUES (1,1,1,'Hero'),(2,1,2,NULL)`,
	)
	return db
}

// TestGenreQuery checks the genre average and the minimum-count filter.
func TestGenreQuery(t *testing.T) {
	db := testDB(t)
	var genre string
	var movieCount int
	var averageRank float64
	if err := db.QueryRow(genreQuery, 2).Scan(&genre, &movieCount, &averageRank); err != nil {
		t.Fatal(err)
	}
	if genre != "Action" || movieCount != 2 || averageRank != 7.65 {
		t.Fatal(genre, movieCount, averageRank)
	}
	if err := db.QueryRow(genreQuery, 3).Scan(&genre, &movieCount, &averageRank); err != sql.ErrNoRows {
		t.Fatalf("genres below the minimum should be excluded, got %v", err)
	}
}

// menuOutput runs the menu with scripted input and returns everything it
// printed, including log lines. Tests using it must not run in parallel
// because it temporarily replaces os.Stdout.
func menuOutput(t *testing.T, db *sql.DB, input string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "output-*")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	originalStdout, originalLog := os.Stdout, log.Writer()
	var logs bytes.Buffer
	os.Stdout = file
	log.SetOutput(&logs)
	defer func() {
		os.Stdout = originalStdout
		log.SetOutput(originalLog)
	}()

	run(db, bufio.NewScanner(strings.NewReader(input)))

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	printed, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(printed) + logs.String()
}

// requireOutput fails the test if any expected text is missing.
func requireOutput(t *testing.T, output string, expected ...string) {
	t.Helper()
	for _, want := range expected {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in output:\n%s", want, output)
		}
	}
}

// TestMenuQueries runs options 1, 2, 3 and 5 against synthetic data.
func TestMenuQueries(t *testing.T) {
	db := testDB(t)
	output := menuOutput(t, db, "1\n2\nspider-man\n3\n1\n3\n2\n3\n999\n5\n0\n")
	requireOutput(t, output, "average_rank", "Spider-Man 2", "Spider-Man | 2002", "Test | Actor | Hero",
		"(not recorded)", "No matching records.", "personal_rating", "Rows: 2")
}

// TestCollectionLifecycle adds, updates and adds an unrated collection item.
func TestCollectionLifecycle(t *testing.T) {
	db := testDB(t)
	output := menuOutput(t, db, "4\n1\nLiving Room\n8.5\n5\n4\n1\nHome Theater\n9\n5\n4\n2\nShelf\n\n5\n0\n")
	requireOutput(t, output, "Collection saved.", "Living Room | 8.5", "Home Theater | 9", "Shelf | (not rated)")

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM collection WHERE movie_id=1").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one updated record, got %d", count)
	}
	var location string
	var rating float64
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

// TestMenuInvalidInput checks every validation message and confirms that
// invalid input never changes the collection.
func TestMenuInvalidInput(t *testing.T) {
	db := testDB(t)
	input := "9\n2\n \n3\n-1\n4\nabc\n4\n1\n \n" +
		"4\n1\nShelf\n11\n4\n1\nShelf\n-1\n4\n1\nShelf\nabc\n4\n1\nShelf\nNaN\n" +
		"4\n99999999\n0\n"
	output := menuOutput(t, db, input)
	requireOutput(t, output, "Choose a number from 0 to 5.", "Enter a movie title.", "enter a positive whole number",
		"Location cannot be empty.", "Rating must be between 0 and 10.",
		"No movie with ID 99999999. Use option 2 to find a movie ID.")
	if strings.Count(output, "Rating must be between 0 and 10.") != 4 {
		t.Error("expected four invalid rating messages")
	}
	if strings.Contains(output, "Constraint Error") {
		t.Error("a technical database error was shown to the user")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM collection").Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid inputs changed collection: %d %v", count, err)
	}
}

// TestMenuEndOfInput checks that the menu stops cleanly when input ends at
// any prompt.
func TestMenuEndOfInput(t *testing.T) {
	db := testDB(t)
	for _, input := range []string{"", "2\n", "3\n", "4\n", "4\n1\n", "4\n1\nShelf\n"} {
		t.Run(input, func(t *testing.T) { requireOutput(t, menuOutput(t, db, input), "Choice:") })
	}
}

// TestMenuLogsDatabaseError checks that a failed query is logged and the
// menu keeps running.
func TestMenuLogsDatabaseError(t *testing.T) {
	db := testDB(t)
	execAll(t, db, "DROP TABLE collection")
	output := menuOutput(t, db, "5\n0\n")
	requireOutput(t, output, "Operation failed:", "Choice:")
}

// TestPrintQueryError checks that SQL errors are returned to the caller.
func TestPrintQueryError(t *testing.T) {
	db := testDB(t)
	if err := printQuery(db, "SELECT * FROM missing_table"); err == nil {
		t.Fatal("expected SQL error")
	}
}

// benchmarkDB generates 10,000 synthetic movies spread across four genres
// and 2000-2004, so benchmarks measure realistic query work.
func benchmarkDB(b *testing.B) *sql.DB {
	b.Helper()
	db := emptyDB(b)
	execAll(b, db,
		`INSERT INTO movies SELECT i, 'Movie ' || i, 2000 + i % 5, (i % 100) / 10.0 FROM range(1, 10001) t(i)`,
		`INSERT INTO genres SELECT i, ['Drama','Comedy','Action','Horror'][i % 4 + 1] FROM range(1, 10001) t(i)`,
	)
	return db
}

// silenceOutput hides printed rows and logs so benchmark results stay readable.
func silenceOutput(b *testing.B) {
	b.Helper()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatal(err)
	}
	originalStdout, originalLog := os.Stdout, log.Writer()
	os.Stdout = devNull
	log.SetOutput(io.Discard)
	b.Cleanup(func() {
		os.Stdout = originalStdout
		log.SetOutput(originalLog)
		devNull.Close()
	})
}

// BenchmarkGenreQuery measures time and memory for the genre ranking.
func BenchmarkGenreQuery(b *testing.B) {
	db := benchmarkDB(b)
	silenceOutput(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := printQuery(db, genreQuery, minGenreMovies); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearchQuery measures time and memory for a title search.
func BenchmarkSearchQuery(b *testing.B) {
	db := benchmarkDB(b)
	silenceOutput(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := printQuery(db, searchQuery, "Movie 99"); err != nil {
			b.Fatal(err)
		}
	}
}
