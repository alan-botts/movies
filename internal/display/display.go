package display

import (
	"fmt"
	"strings"

	"github.com/alan-botts/movies/internal/showtimes"
)

// PrintShowtimes prints movie showtime results in a readable plain-text format.
func PrintShowtimes(movies []showtimes.Movie, zip, date string) {
	fmt.Printf("Found %d movies showing near %s on %s\n", len(movies), zip, date)
	fmt.Println(strings.Repeat("=", 60))

	for i, movie := range movies {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("\n%s\n", movie.Title)
		fmt.Println(strings.Repeat("-", len(movie.Title)))

		if len(movie.Theaters) == 0 {
			fmt.Println("  No theaters found.")
			continue
		}

		for _, theater := range movie.Theaters {
			fmt.Printf("\n  %s", theater.Name)
			if theater.Distance != "" {
				fmt.Printf("  (%s)", theater.Distance)
			}
			fmt.Println()
			if theater.Address != "" {
				fmt.Printf("  %s\n", theater.Address)
			}

			if len(theater.Showings) == 0 {
				fmt.Println("  No showtimes available.")
				continue
			}

			timesStr := showtimes.FormatShowings(theater.Showings)
			fmt.Printf("  Times: %s\n", timesStr)
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("Data from SerpApi (Google Showtimes). %d movies, %d theaters total.\n",
		len(movies), countTheaters(movies))
}

func countTheaters(movies []showtimes.Movie) int {
	seen := make(map[string]bool)
	for _, m := range movies {
		for _, t := range m.Theaters {
			seen[t.Name] = true
		}
	}
	return len(seen)
}
