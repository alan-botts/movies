# movies

A Go CLI tool that searches for movie showtimes near a US zip code.

## Data Source

Uses [SerpApi](https://serpapi.com/) to query Google's showtime results. SerpApi has a free tier (250 searches/month).

### Setup

1. Sign up at https://serpapi.com (free tier available)
2. Copy your API key from the dashboard
3. Export it:

```bash
export SERPAPI_API_KEY=your_key_here
```

Or store it with goat creds:

```bash
./goat creds set SERPAPI_API_KEY your_key_here
```

## Build

```bash
go build -o movies .
```

## Usage

```bash
# Search showtimes near a zip code (defaults to today, 20mi radius)
./movies showtimes 94703

# Specify date and radius
./movies showtimes 94703 --radius 20 --date 2026-04-17

# Plain text output (for piping / agent use)
./movies showtimes 94703 --headless
```

## Output

Movies are grouped by title. Under each movie, theaters are listed with:
- Theater name and distance from zip
- Theater address
- Showtimes grouped by format (Standard, IMAX, Dolby Cinema, etc.)

## Project Structure

```
movies/
├── cmd/
│   └── movies/
│       ├── root.go         # Root cobra command
│       └── showtimes.go    # Showtimes subcommand
├── internal/
│   ├── showtimes/
│   │   └── provider.go     # SerpApi data provider
│   └── display/
│       └── display.go      # Output formatting
├── main.go                 # Entry point
├── go.mod
└── README.md
```

## API Response

SerpApi returns showtimes in two possible formats:
1. **Movie-centric**: Movies at the top level, theaters nested under each movie
2. **Theater-centric**: Theaters at the top level, movies nested under each theater

The provider handles both formats and normalizes them into a movie-grouped structure.
