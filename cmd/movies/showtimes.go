package movies

import (
	"fmt"
	"regexp"
	"time"

	"github.com/spf13/cobra"

	"github.com/alan-botts/movies/internal/display"
	"github.com/alan-botts/movies/internal/showtimes"
)

var (
	radius   int
	date     string
	headless bool
)

var showtimesCmd = &cobra.Command{
	Use:   "showtimes <zip>",
	Short: "Search for movie showtimes near a zip code",
	Long:  "Search for movie showtimes at theaters near the given US zip code. Uses the SerpApi Google Showtimes API.",
	Args:  cobra.ExactArgs(1),
	RunE:  runShowtimes,
}

func init() {
	showtimesCmd.Flags().IntVar(&radius, "radius", 20, "Search radius in miles")
	showtimesCmd.Flags().StringVar(&date, "date", "", "Date to search (YYYY-MM-DD, defaults to today)")
	showtimesCmd.Flags().BoolVar(&headless, "headless", false, "Plain text output (no TUI)")
	rootCmd.AddCommand(showtimesCmd)
}

func runShowtimes(cmd *cobra.Command, args []string) error {
	zip := args[0]

	// Validate zip code (5 digits).
	zipRe := regexp.MustCompile(`^\d{5}$`)
	if !zipRe.MatchString(zip) {
		return fmt.Errorf("invalid zip code: %s (expected 5-digit US zip code)", zip)
	}

	// Default date to today.
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// Validate date format.
	dateRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if !dateRe.MatchString(date) {
		return fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD)", date)
	}

	fmt.Printf("Searching showtimes near %s (radius: %d mi, date: %s)...\n\n", zip, radius, date)

	provider, err := showtimes.NewSerpApiProvider()
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	results, err := provider.Search(zip, date)
	if err != nil {
		return fmt.Errorf("showtime search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No showtimes found.")
		return nil
	}

	display.PrintShowtimes(results, zip, date)
	return nil
}
