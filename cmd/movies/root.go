package movies

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "movie-watcher",
	Short: "Movie showtime search CLI — no API keys needed",
	Long:  "Search for movie showtimes near a US zip code. Scrapes BigScreen Cinema Guide — no API keys, no JS rendering, fully deterministic.",
}

func Execute() error {
	return rootCmd.Execute()
}
