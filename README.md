# movie-watcher

A Go CLI tool that searches for movie showtimes near a US zip code. No API keys needed.

Part of the **alan-botts tools** family:
- [traveler](https://github.com/alan-botts/traveler) — flight search via Google Flights
- [divine](https://github.com/alan-botts/divine) — divination CLI (tarot, I Ching, runes, koans)
- **movie-watcher** — movie showtime search (you are here)

All tools listed at [strangerloops.com/tools](https://strangerloops.com/tools/).

## Data Source

Scrapes **BigScreen Cinema Guide** ([bigscreen.com](https://www.bigscreen.com))
printable showtime pages. No API keys needed, no JavaScript rendering, no
headless browser — just plain HTTP requests to deterministic, static HTML pages.

## Build

```bash
go build -o movie-watcher .
```

## Usage

```bash
# Search showtimes near a zip code (defaults to today, 20mi radius)
./movie-watcher showtimes 94703

# Specify date and radius
./movie-watcher showtimes 94703 --radius 30 --date 2026-04-17

# List theaters near a zip code
./movie-watcher theaters 94703 --radius 30

# Dump all known theaters as JSON
./movie-watcher theaters --all
```

## Output

Movies are grouped by title. Under each movie, theaters are listed with:
- Theater name and city
- Showtimes (AM times suffixed with "a", PM times without)
- Features (Stadium Seating, IMAX, Digital Projection, etc.)

## How It Works

1. Finds all known theaters within the search radius using haversine distance from the zip code
2. Fetches the BigScreen printable showtime page for each theater (concurrent, up to 5 at a time)
3. Parses the HTML to extract movie titles, ratings, runtimes, and showtimes
4. Merges results: groups by movie title across all theaters
5. Returns sorted by movie title

## Theater Coverage

The hardcoded theater database includes 45+ theaters covering:
- **East Bay**: Berkeley, Oakland, Emeryville, Alameda, El Cerrito, Richmond
- **Tri-Valley**: Walnut Creek, Concord, Pleasant Hill, Lafayette, Orinda, Moraga, Danville, Dublin, Pleasanton, Livermore
- **South Bay (partial)**: Fremont, Hayward, Castro Valley, Union City, San Leandro, Newark
- **Marin**: Larkspur, Mill Valley, Tiburon, Novato
- **Central Valley**: Stockton, Tracy, Manteca, Lodi, Modesto
- **Sacramento**: Downtown, midtown theaters

To add a theater, find its BigScreen ID from [bigscreen.com](https://www.bigscreen.com) and add it to `internal/showtimes/theaters.go` with lat/lon coordinates.

## License

MIT License — see [LICENSE](LICENSE).
