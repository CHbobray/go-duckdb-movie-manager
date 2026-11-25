# Personal Movie Collection Manager

MSDS 431 — Data Engineering with Go | Northwestern University

Repository: https://github.com/CHbobray/go-duckdb-movie-manager.git

## Project overview
This project explores how Python and Go can work together through DuckDB. Python reads four IMDb CSV files into Polars DataFrames and stores them in a relational database. Go provides a command-line application for searching movies, comparing genre ratings, finding cast members, and managing a personal collection.

## Repository organization
- `data/`: four course-provided CSV files
- `schema.sql`: relational schema and constraints
- `load_data.py`: Python/Polars database loader
- `requirements.txt`: Python dependencies
- `go.mod`: Go dependency declaration
- `main.go`: Go application and SQL queries
- `main_test.go`: validation and synthetic database tests
- `go.sum`: Go dependency checksums
- `coverage.out` and `coverage.html`: automated coverage reports
- `GenAI_Log.md`: summary of AI assistance

## Data preparation and schema
The source contains 11,502 movies, 130,002 actors, 18,548 genre records and 203,027 roles. Movies span 2000–2004. Movie and actor IDs connect the tables. Movies and actors have primary keys; genres use a composite key; roles use a generated ID because an actor can play multiple roles in a movie. Missing role descriptions (17,386) remain null. No exact duplicate rows or orphaned foreign keys were observed in the source inspection. The original rank field is retained. The collection table adds a movie ID, storage location and optional personal rating. DuckDB, rather than SQLite, is used because the main assignment specifies DuckDB; SQLite is an optional comparison.

## Setup and running
I tested the application on 64-bit Windows with Python 3.14, DuckDB 1.5.6, Polars 1.44.2, PyArrow 25.0.1, Go 1.27.1, and the WinLibs GCC 14.2.0 compiler. Git and an internet connection are needed to clone the repository and install dependencies. The Go driver is `github.com/marcboeker/go-duckdb` v1.8.5 and requires CGO.

### Clone the repository

```powershell
git clone https://github.com/CHbobray/go-duckdb-movie-manager.git
cd go-duckdb-movie-manager
```

Place the four course-provided CSV files in `data/` if they are not included in the clone:

- `aan520_w5a_movies2000After.csv`
- `aan520_w5a_actors2000After.csv`
- `aan520_w5a_genres2000After.csv`
- `aan520_w5a_roles2000After.csv`

Obtain these edited files from the course assignment. The Northwestern archive is listed under References; check redistribution permissions before publishing the CSV files.

### Windows compiler setup

I initially installed GCC 16.2.0 through MSYS2. Although CGO was enabled, the DuckDB driver failed during linking with undefined `__emutls` references. Switching to WinLibs GCC 14.2.0 (x86_64, UCRT, POSIX, SEH) resolved the problem. The original database and application code did not need to change.

Download and extract the matching compiler from https://winlibs.com/ or https://github.com/brechtsanders/winlibs_mingw/releases. If extracted under Downloads, use PowerShell to locate it and configure the current session:

```powershell
$gcc = Get-ChildItem "$HOME\Downloads" -Filter gcc.exe -Recurse -ErrorAction SilentlyContinue | Where-Object { $_.FullName -like "*gcc-14.2.0*" } | Select-Object -First 1
if (-not $gcc) { throw "Locate the extracted GCC 14.2.0 compiler first." }
$env:Path = "$($gcc.DirectoryName);$env:Path"
$env:CC = $gcc.FullName
$env:CXX = Join-Path $gcc.DirectoryName "g++.exe"
$env:CGO_ENABLED = "1"
gcc --version
go env CC CXX CGO_ENABLED
```

These settings apply to the current PowerShell session. Adjust the search directory if WinLibs is installed elsewhere. If switching from another compiler, run `go clean -cache` before testing. Use a compatible CGO toolchain on macOS or Linux; those platforms were not tested in this project.

### Install dependencies and run

From the repository root:

```powershell
python -m pip install -r requirements.txt
python load_data.py
go mod tidy
go test -v ./...
go run .
```

The loader creates `movies.duckdb` and refuses to overwrite an existing database to protect collection entries. Skip the loader if a valid database already exists. Back up any database before rebuilding it. Close other database connections before starting the application.

### Menu instructions

| Choice | Action |
|---|---|
| 1 | Display genre rankings |
| 2 | Search by movie title (up to 25 matches) |
| 3 | Enter a movie ID to find cast and roles |
| 4 | Enter a movie ID, storage location and optional rating from 0 to 10 |
| 5 | Display the personal collection |
| 0 | Exit |

For example, search for Spider-Man, use movie ID `311037` to view its cast, and then add it to the collection. Selecting option 4 again with the same ID updates the existing record.

## Build an executable
macOS/Linux: `CGO_ENABLED=1 go build -o movie-manager .` then `./movie-manager`.
Windows PowerShell: `$env:CGO_ENABLED="1"` then `go build -o movie-manager.exe .` and `.\movie-manager.exe`.
The macOS output is a native command-line executable, not a GUI application bundle. If an `.app` bundle is required, run `mkdir -p MovieManager.app/Contents/MacOS` and copy the compiled binary there; invoke the binary from Terminal to retain interactive input. The database path can be passed as an argument. Build on the target platform with a compatible toolchain.

## SQL analysis
The genre query joins movies and genres using movie_id, calculates average IMDb rank and excludes genres with fewer than 20 movies to reduce the effect of small samples. Movies with multiple genres contribute to each relevant genre.

| Genre | Movie count | Average rank |
|---|---:|---:|
| Documentary | 1,047 | 6.98 |
| Short | 2,055 | 6.84 |
| War | 154 | 6.64 |
| Animation | 416 | 6.55 |
| Music | 112 | 6.42 |
| Drama | 4,498 | 6.27 |
| Romance | 1,230 | 6.12 |
| Mystery | 300 | 6.05 |
| Comedy | 3,262 | 6.03 |
| Musical | 200 | 5.97 |

The Go application returned 19 genres. The ten highest are shown above. Documentary ranked first, followed by Short and War. Horror ranked last among the returned genres at 5.25. Drama had the largest count (4,498) but ranked sixth. This shows that a larger number of movies does not necessarily mean a higher average rating. The results describe the historical dataset, not the current IMDb catalog.

```sql
SELECT g.genre, COUNT(*) AS movie_count, ROUND(AVG(m.rank), 2) AS average_rank
FROM movies m JOIN genres g ON m.movie_id = g.movie_id
GROUP BY g.genre HAVING COUNT(*) >= 20
ORDER BY average_rank DESC, g.genre;
```

## Testing and software metrics
I ran the original two tests and found that both passed, but statement coverage was only 4.0%. I expanded the suite to seven test functions using synthetic data in an isolated in-memory DuckDB database. All tests passed and coverage increased to 85.1%. The expanded suite took 0.333 seconds on my computer.

| Function | Statement coverage |
|---|---:|
| `positiveInt` | 100.0% |
| `printQuery` | 90.6% |
| `run` | 100.0% |
| `main` | 0.0% |
| Total | 85.1% |

The tests exercise the underlying functions directly rather than calling `main`. I separately verified startup and exit using both `go run .` and the Windows executable. Coverage measures executed statements, not proof that every possible input works.

### Reproduce the tests and coverage report

```powershell
go test -v ./...
go test -cover ./...
go test "-coverprofile=coverage.out" ./...
go tool cover "-func=coverage.out"
go tool cover "-html=coverage.out" -o coverage.html
Start-Process coverage.html
```

The quotation marks prevent Windows PowerShell from splitting the coverage filename argument.

### Manual functional tests

I completed these 12 planned checks using the actual database. All passed.

| Test | Observed result |
|---|---|
| Genre rankings | 19 genres returned |
| Movie search | Spider-Man (2002) and Spider-Man 2 (2004) returned |
| Cast lookup | 100 cast/role records returned for movie 311037 |
| Add collection item | Spider-Man saved with rating 8.5 |
| View collection | Correct movie, location and rating returned |
| Update collection | Location changed to Home Theater - Blu-ray and rating to 9; still one record |
| No matching records | ID 123 returned no cast records |
| Negative ID | -1 rejected |
| Nonexistent collection movie | ID 99999999 rejected by foreign key constraint |
| Invalid rating | 11 rejected |
| Invalid menu choice | 9 rejected |
| Exit | Returned to PowerShell |

### Actual performance results (November 24, 2026)

| Operation | Elapsed time | Reported memory |
|---|---:|---:|
| Python load (363,079 source records) | 13.945 s | 35.91 MiB |
| Original Go tests | 0.364 s | — |
| Expanded Go tests | 0.333 s | — |
| Genre ranking | 36.2921 ms | 0.36 MiB |
| Movie search | 25.93 ms | 0.38 MiB |
| Cast lookup | 65.8221 ms | 0.42 MiB |
| Initial collection retrieval | 24.8047 ms | 0.43 MiB |
| No matching cast records | 4.6541 ms | 0.43 MiB |
| Updated collection retrieval, first run | 9.8561 ms | 0.44 MiB |
| Updated collection retrieval, second run | 8.1735 ms | 0.44 MiB |

The Windows executable was 59,066,020 bytes (approximately 59.07 MB) and launched successfully. Python reports elapsed wall time and peak Python-tracked allocations. Go reports query/display elapsed time and current Go heap allocation, not peak or total process memory. Neither includes all native database allocations. These are observed runs, not a controlled benchmark or a comparison of Python and Go performance.

## Personal collection and application design
I would use the application to keep track of movies I own, where they are stored and how I rate them. For example, I could search for a movie, locate its ID, add it to my collection and record whether it is on a DVD shelf or in a digital library. IMDb provides general movie information, but this application lets me organize my own collection and compare my ratings with IMDb ranks. The menu provides a straightforward interface without requiring the user to write SQL.

## Future development
I would add release data after 2004, a watchlist, multiple storage locations, viewing dates, and a browser-based interface. A future recommendation system could use a user table (user_id), a reviews table (user_id, movie_id, rating, review_text, reviewed_at), and a viewing_history table (user_id, movie_id, watched_at). Genre preferences and personal ratings could support content-based recommendations. More users and their ratings would be needed for collaborative filtering. I did not perform the optional Python/Go or SQLite performance comparisons. I would also improve the technical foreign key error message by displaying a simple message telling the user that the movie ID does not exist.

## GenAI Tools
I used ChatGPT to help inspect the CSV files, design the schema, draft the Python loader and Go application, develop and expand automated tests, troubleshoot Windows compiler and PowerShell errors, and prepare the documentation. I ran the application and tests on my Windows computer and supplied screenshots of the actual results. The first draft could not be executed in the AI drafting environment, but I verified the results locally. See [GenAI_Log.md](GenAI_Log.md) for a summary of the assistance. The actual conversation transcript or share link must also be included for the assignment; see the instructions in that file.

## References
- Course-provided Northwestern IMDb files: https://arch.library.northwestern.edu/concern/datasets/3484zh40n?locale=en
- Original dataset: https://relational.fit.cvut.cz/dataset/IMDb
- DuckDB: https://duckdb.org/docs/
- Polars: https://docs.pola.rs/
- Go: https://go.dev/doc/
