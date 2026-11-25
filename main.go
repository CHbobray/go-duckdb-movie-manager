package main

import (
	"bufio"
	"database/sql"
	"fmt"
	_ "github.com/marcboeker/go-duckdb"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const genreQuery = `SELECT g.genre, COUNT(*) AS movie_count, ROUND(AVG(m.rank), 2) AS average_rank
FROM movies m JOIN genres g ON m.movie_id = g.movie_id
GROUP BY g.genre HAVING COUNT(*) >= ? ORDER BY average_rank DESC, g.genre`
const searchQuery = `SELECT movie_id, name, year, rank FROM movies WHERE name ILIKE '%' || ? || '%' ORDER BY rank DESC, movie_id LIMIT 25`
const castQuery = `SELECT a.first_name, a.last_name, COALESCE(r.role, '(not recorded)') FROM actors a JOIN roles r ON a.actor_id=r.actor_id WHERE r.movie_id=? ORDER BY a.last_name, a.first_name LIMIT 100`
const collectionQuery = `SELECT m.movie_id, m.name, c.location, c.personal_rating FROM collection c JOIN movies m ON c.movie_id=m.movie_id ORDER BY m.name`

func positiveInt(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("enter a positive whole number")
	}
	return n, nil
}

// printQuery displays arbitrary query results and records elapsed time.
func printQuery(db *sql.DB, query string, args ...any) error {
	start := time.Now()
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(cols, " | "))
	count := 0
	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return err
		}
		parts := make([]string, len(cols))
		for i, v := range values {
			if v == nil {
				parts[i] = "(not rated)"
			} else {
				parts[i] = fmt.Sprint(v)
			}
		}
		fmt.Println(strings.Join(parts, " | "))
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count == 0 {
		fmt.Println("No matching records.")
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	log.Printf("Rows: %d; elapsed: %s; Go heap: %.2f MiB", count, time.Since(start), float64(mem.Alloc)/1048576)
	return nil
}

func run(db *sql.DB, in *bufio.Scanner) {
	for {
		fmt.Println("\n1. Highest-rated genres\n2. Search movies\n3. Find movie cast\n4. Add or update collection item\n5. View collection\n0. Exit")
		fmt.Print("Choice: ")
		if !in.Scan() {
			return
		}
		choice := strings.TrimSpace(in.Text())
		var err error
		switch choice {
		case "0":
			return
		case "1":
			err = printQuery(db, genreQuery, 20)
		case "2":
			fmt.Print("Movie title: ")
			if !in.Scan() {
				return
			}
			term := strings.TrimSpace(in.Text())
			if term == "" {
				fmt.Println("Enter a movie title.")
				continue
			}
			err = printQuery(db, searchQuery, term)
		case "3", "4":
			fmt.Print("Movie ID: ")
			if !in.Scan() {
				return
			}
			id, parseErr := positiveInt(in.Text())
			if parseErr != nil {
				fmt.Println(parseErr)
				continue
			}
			if choice == "3" {
				err = printQuery(db, castQuery, id)
				break
			}
			fmt.Print("Collection location: ")
			if !in.Scan() {
				return
			}
			location := strings.TrimSpace(in.Text())
			if location == "" {
				fmt.Println("Location cannot be empty.")
				continue
			}
			fmt.Print("Personal rating (0-10, blank if unrated): ")
			if !in.Scan() {
				return
			}
			var rating any
			raw := strings.TrimSpace(in.Text())
			if raw != "" {
				v, e := strconv.ParseFloat(raw, 64)
				if e != nil || !(v >= 0 && v <= 10) {
					fmt.Println("Rating must be between 0 and 10.")
					continue
				}
				rating = v
			}
			_, err = db.Exec(`INSERT INTO collection VALUES (?, ?, ?) ON CONFLICT(movie_id) DO UPDATE SET location=excluded.location, personal_rating=excluded.personal_rating`, id, location, rating)
			if err == nil {
				fmt.Println("Collection saved.")
			}
		case "5":
			err = printQuery(db, collectionQuery)
		default:
			fmt.Println("Choose a number from 0 to 5.")
		}
		if err != nil {
			log.Printf("Operation failed: %v", err)
		}
	}
}

func main() {
	path := "movies.duckdb"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if _, err := os.Stat(path); err != nil {
		log.Fatalf("Database unavailable: %v. Run python load_data.py first.", err)
	}
	db, err := sql.Open("duckdb", path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	run(db, bufio.NewScanner(os.Stdin))
}
