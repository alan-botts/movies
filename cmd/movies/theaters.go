package movies

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/alan-botts/movies/internal/showtimes"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "theaters [zip]",
		Short: "List theaters near a zip code or dump all known theaters",
		Long:  "List theaters within a radius of a zip code, or use --all to dump the full theater database as JSON.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runTheaters,
	}
	cmd.Flags().Int("radius", 30, "Search radius in miles")
	cmd.Flags().Bool("all", false, "Dump all known theaters as JSON")
	cmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(cmd)
}

type theaterOutput struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	City     string  `json:"city"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Distance float64 `json:"distance_mi,omitempty"`
	URL      string  `json:"bigscreen_url"`
}

func runTheaters(cmd *cobra.Command, args []string) error {
	allFlag, _ := cmd.Flags().GetBool("all")
	jsonFlag, _ := cmd.Flags().GetBool("json")
	radius, _ := cmd.Flags().GetInt("radius")

	theaters := showtimes.KnownTheaters

	if allFlag {
		out := make([]theaterOutput, len(theaters))
		for i, t := range theaters {
			out[i] = theaterOutput{
				ID:   t.ID,
				Name: t.Name,
				City: t.City,
				Lat:  t.Lat,
				Lon:  t.Lon,
				URL:  fmt.Sprintf("https://www.bigscreen.com/Marquee.php?theater=%d", t.ID),
			}
		}
		if jsonFlag {
			return writeJSON(out)
		}
		fmt.Printf("All %d known theaters:\n\n", len(out))
		for _, t := range out {
			fmt.Printf("  ID %-6d  %-40s  %s  (%.4f, %.4f)\n", t.ID, t.Name, t.City, t.Lat, t.Lon)
		}
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("provide a zip code, or use --all to list all theaters")
	}

	zip := args[0]
	lat, lon, err := showtimes.ZipToLatLon(zip)
	if err != nil {
		return fmt.Errorf("unknown zip code %q — add it to the zip database", zip)
	}

	var results []theaterOutput
	for _, t := range theaters {
		dist := haversineMi(lat, lon, t.Lat, t.Lon)
		if dist <= float64(radius) {
			results = append(results, theaterOutput{
				ID:       t.ID,
				Name:     t.Name,
				City:     t.City,
				Lat:      t.Lat,
				Lon:      t.Lon,
				Distance: math.Round(dist*10) / 10,
				URL:      fmt.Sprintf("https://www.bigscreen.com/Marquee.php?theater=%d", t.ID),
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})

	if jsonFlag {
		return writeJSON(results)
	}

	fmt.Printf("Theaters within %d miles of %s (%d found):\n\n", radius, zip, len(results))
	for _, t := range results {
		fmt.Printf("  %4.1f mi  ID %-6d  %-40s  %s\n", t.Distance, t.ID, t.Name, t.City)
	}
	return nil
}

func writeJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func haversineMi(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 3958.8
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
