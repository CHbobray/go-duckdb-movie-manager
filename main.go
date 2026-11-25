// Command movie-manager is a menu-driven application for exploring the IMDb
// movie database created by load_data.py and managing a personal collection.
package main

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb" // registers the "duckdb" database/sql driver
)

// minGenreMovies excludes genres with too few movies for a meaningful average.
const minGenreMovies = 20

// genreQuery ranks genres by average IMDb rank. A movie with several genres
// counts toward each of them.
const genreQuery = `SELECT g.genre, COUNT(*) AS movie_count, ROUND(AVG(m.rank), 2) AS average_rank
FROM movies m JOIN genres g ON m.movie_id = g.movie_id
GROUP BY g.genre HAVING COUNT(*) >= ? ORDER BY average_rank DESC, g.genre`

// searchQuery finds up to 25 movies whose title contains the search term,
// ignoring case, with the highest-ranked movies first.
const searchQuery = `SELECT movie_id, name, year, rank FROM movies
WHERE name ILIKE '%' || ? || '%'
ORDER BY rank DESC, movie_id LIMIT 25`

// castQuery lists up to 100 actors and their roles for one movie.
// Missing role descriptions are shown as "(not recorded)".
const castQuery = `SELECT a.first_name, a.last_name, COALESCE(r.role, '(not recorded)') AS role
FROM actors a JOIN roles r ON a.actor_id = r.actor_id
WHERE r.movie_id = ? ORDER BY a.last_name, a.first_name LIMIT 100`

// collectionQuery lists the personal collection alphabetically by title.
const collectionQuery = `SELECT m.movie_id, m.name, c.location, c.personal_rating
FROM collection c JOIN movies m ON c.movie_id = m.movie_id ORDER BY m.name`

// movieExistsQuery checks that a movie ID is valid before it is saved.
const movieExistsQuery = `SELECT EXISTS (SELECT 1 FROM movies WHERE movie_id = ?)`

// saveCollectionQuery adds a movie to the collection, or updates the location
// and rating if the movie is already there, so each movie appears only once.
const saveCollectionQuery = `INSERT INTO collection VALUES (?, ?, ?)
ON CONFLICT (movie_id) DO UPDATE SET location = excluded.location, personal_rating = excluded.personal_rating`

// menuText lists the choices shown before each prompt.
const menuText = `
1. Highest-rated genres
2. Search movies
3. Find movie cast
4. Add or update collection item
5. View collection
0. Exit`

// errInputEnded signals that standard input closed, so the menu should stop.
var errInputEnded = errors.New("input ended")

// positiveInt converts user text to a whole number greater than zero.
func positiveInt(text string) (int, error) {
	number, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("enter a positive whole number")
	}
	return number, nil
}

// parseRating converts user text to a rating from 0 to 10.
// Blank text returns nil, which is stored as NULL (not rated).
func parseRating(text string) (any, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	rating, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(rating) || rating < 0 || rating > 10 {
		return nil, fmt.Errorf("rating must be between 0 and 10")
	}
	return rating, nil
}

// formatValue turns a database value into display text. NULL appears only in
// the personal_rating column, so it is shown as "(not rated)".
func formatValue(value any) string {
	if value == nil {
		return "(not rated)"
	}
	return fmt.Sprint(value)
}

// printQuery runs any SELECT statement and prints its rows as a
// pipe-separated table. It works with any number of columns, so it can be
// reused for other queries and databases. Elapsed time, row count and current
// Go heap use are written to the log.
func printQuery(db *sql.DB, query string, args ...any) error {
	start := time.Now()
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(columns, " | "))

	// Scan into generic values because the column types differ by query.
	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	rowCount := 0
	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return err
		}
		cells := make([]string, len(values))
		for i, value := range values {
			cells[i] = formatValue(value)
		}
		fmt.Println(strings.Join(cells, " | "))
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if rowCount == 0 {
		fmt.Println("No matching records.")
	}

	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	log.Printf("Rows: %d; elapsed: %s; Go heap: %.2f MiB", rowCount, time.Since(start), float64(memory.Alloc)/(1<<20))
	return nil
}

// menu holds the database connection and the source of user input.
type menu struct {
	db    *sql.DB
	input *bufio.Scanner
}

// prompt shows a label and returns the trimmed line the user typed.
// It returns errInputEnded when no more input is available.
func (m menu) prompt(label string) (string, error) {
	fmt.Print(label)
	if !m.input.Scan() {
		return "", errInputEnded
	}
	return strings.TrimSpace(m.input.Text()), nil
}

// promptMovieID asks for a movie ID. ok is false when the user typed an
// invalid ID; the explanation has already been printed.
func (m menu) promptMovieID() (id int, ok bool, err error) {
	text, err := m.prompt("Movie ID: ")
	if err != nil {
		return 0, false, err
	}
	id, parseErr := positiveInt(text)
	if parseErr != nil {
		fmt.Println(parseErr)
		return 0, false, nil
	}
	return id, true, nil
}

// showGenres prints the genre rankings (menu option 1).
func (m menu) showGenres() error {
	return printQuery(m.db, genreQuery, minGenreMovies)
}

// searchMovies asks for part of a title and prints matching movies (option 2).
func (m menu) searchMovies() error {
	title, err := m.prompt("Movie title: ")
	if err != nil {
		return err
	}
	if title == "" {
		fmt.Println("Enter a movie title.")
		return nil
	}
	return printQuery(m.db, searchQuery, title)
}

// showCast asks for a movie ID and prints its actors and roles (option 3).
func (m menu) showCast() error {
	id, ok, err := m.promptMovieID()
	if err != nil || !ok {
		return err
	}
	return printQuery(m.db, castQuery, id)
}

// saveCollectionItem asks for a movie ID, a storage location and an optional
// rating, then adds or updates that movie in the collection (option 4).
// Each input is checked before anything is written.
func (m menu) saveCollectionItem() error {
	id, ok, err := m.promptMovieID()
	if err != nil || !ok {
		return err
	}
	var exists bool
	if err := m.db.QueryRow(movieExistsQuery, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		fmt.Printf("No movie with ID %d. Use option 2 to find a movie ID.\n", id)
		return nil
	}

	location, err := m.prompt("Collection location: ")
	if err != nil {
		return err
	}
	if location == "" {
		fmt.Println("Location cannot be empty.")
		return nil
	}

	ratingText, err := m.prompt("Personal rating (0-10, blank if unrated): ")
	if err != nil {
		return err
	}
	rating, ratingErr := parseRating(ratingText)
	if ratingErr != nil {
		fmt.Println("Rating must be between 0 and 10.")
		return nil
	}

	if _, err := m.db.Exec(saveCollectionQuery, id, location, rating); err != nil {
		return err
	}
	fmt.Println("Collection saved.")
	return nil
}

// showCollection prints the personal collection (option 5).
func (m menu) showCollection() error {
	return printQuery(m.db, collectionQuery)
}

// run shows the menu until the user chooses 0 or input ends. Database errors
// are logged and the menu continues, so one failed action does not end the
// program.
func run(db *sql.DB, input *bufio.Scanner) {
	m := menu{db: db, input: input}
	actions := map[string]func() error{
		"1": m.showGenres,
		"2": m.searchMovies,
		"3": m.showCast,
		"4": m.saveCollectionItem,
		"5": m.showCollection,
	}
	for {
		fmt.Println(menuText)
		choice, err := m.prompt("Choice: ")
		if err != nil || choice == "0" {
			return
		}
		action, found := actions[choice]
		if !found {
			fmt.Println("Choose a number from 0 to 5.")
			continue
		}
		err = action()
		if errors.Is(err, errInputEnded) {
			return
		}
		if err != nil {
			log.Printf("Operation failed: %v", err)
		}
	}
}

// main opens the database (movies.duckdb, or the path given as the first
// argument) and starts the menu.
func main() {
	path := "movies.duckdb"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	// sql.Open would silently create an empty database, so check first.
	if _, err := os.Stat(path); err != nil {
		log.Fatalf("Database unavailable: %v. Run python load_data.py first.", err)
	}
	db, err := sql.Open("duckdb", path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	run(db, bufio.NewScanner(os.Stdin))
}
