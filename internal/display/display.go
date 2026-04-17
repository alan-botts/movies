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
		// Movie title with rating and runtime.
		header := movie.Title
		if movie.Rating != "" {
			header += fmt.Sprintf(" [%s]", movie.Rating)
		}
		if movie.Runtime != "" {
			header += fmt.Sprintf(" (%s)", movie.Runtime)
		}
		fmt.Printf("\n%s\n", header)
		fmt.Println(strings.Repeat("-", len(header)))

		if len(movie.Theaters) == 0 {
			fmt.Println("  No theaters found.")
			continue
		}

		for _, theater := range movie.Theaters {
			fmt.Printf("\n  %s", theater.Name)
			if theater.City != "" {
				fmt.Printf(" — %s", theater.City)
			}
			fmt.Println()

			if len(theater.Showtimes) == 0 {
				fmt.Println("    No showtimes available.")
				continue
			}

			fmt.Printf("    %s\n", strings.Join(theater.Showtimes, ", "))

			if theater.Features != "" {
				fmt.Printf("    [%s]\n", theater.Features)
			}
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("Data from BigScreen Cinema Guide (bigscreen.com). %d movies, %d theaters.\n",
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
