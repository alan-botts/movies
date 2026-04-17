# movies

A Go CLI tool that searches for movie showtimes near a US zip code.

## Data Source

Scrapes **BigScreen Cinema Guide** ([bigscreen.com](https://www.bigscreen.com))
printable showtime pages. No API keys needed, no JavaScript rendering, no
headless browser — just plain HTTP requests to deterministic, static HTML pages.

URL format:
```
https://www.bigscreen.com/Marquee.php?theater={ID}&view=sched&printable=1&showdate={YYYY-MM-DD}
```

## Build

```bash
go build -o movie-cli .
```

## Usage

```bash
# Search showtimes near a zip code (defaults to today, 20mi radius)
./movie-cli showtimes 94703

# Specify date and radius
./movie-cli showtimes 94703 --radius 30 --date 2026-04-17
```

## Output

Movies are grouped by title. Under each movie, theaters are listed with:
- Theater name and city
- Showtimes (AM times suffixed with "a", PM times without)
- Features (Stadium Seating, IMAX, Digital Projection, etc.)

## How It Works

1. Maps the given zip code to lat/lon coordinates (hardcoded database)
2. Finds all known theaters within the search radius using haversine distance
3. Fetches the BigScreen printable showtime page for each theater (concurrent, up to 5 at a time)
4. Parses the HTML to extract movie titles, ratings, runtimes, and showtimes
5. Merges results: groups by movie title across all theaters
6. Returns sorted by movie title

## Theater Coverage

The hardcoded theater database includes 45+ theaters covering:
- **East Bay**: Berkeley, Oakland, Emeryville, Alameda, El Cerrito, Richmond
- **Tri-Valley**: Walnut Creek, Concord, Pleasant Hill, Lafayette, Orinda, Moraga, Danville, Dublin, Pleasanton, Livermore
- **South Bay (partial)**: Fremont, Hayward, Castro Valley, Union City, San Leandro, Newark
- **Marin**: Larkspur, Mill Valley, Tiburon, Novato
- **Central Valley**: Stockton, Tracy, Manteca, Lodi, Modesto
- **Sacramento**: Downtown, midtown theaters

## Project Structure

```
movies/
├── cmd/
│   └── movies/
│       ├── root.go         # Root cobra command
│       └── showtimes.go    # Showtimes subcommand
├── internal/
│   ├── showtimes/
│   │   ├── provider.go     # BigScreen scraper (plain HTTP + HTML parsing)
│   │   └── theaters.go     # Hardcoded theater database with lat/lon
│   └── display/
│       └── display.go      # Output formatting
├── main.go                 # Entry point
├── go.mod
└── README.md
```
