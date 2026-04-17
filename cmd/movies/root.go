package movies

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "movies",
	Short: "Movie showtime search CLI",
	Long:  "A CLI tool that searches for movie showtimes near a given zip code using the SerpApi Google Showtimes API.",
}

func Execute() error {
	return rootCmd.Execute()
}
