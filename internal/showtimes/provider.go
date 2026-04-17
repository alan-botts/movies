package showtimes

import (
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// Movie represents a movie with its showtimes across theaters.
type Movie struct {
	Title    string
	Rating   string // PG-13, R, Not Rated, etc.
	Runtime  string // "1:35"
	Theaters []TheaterShowtime
}

// TheaterShowtime holds showtime data for one movie at one theater.
type TheaterShowtime struct {
	TheaterID int
	Name      string
	City      string
	Showtimes []string // ["1:30", "6:15", "8:30"]
	Features  string   // "Stadium Seating; Digital Projection"
}

// BigScreenClient fetches showtimes from BigScreen Cinema Guide.
type BigScreenClient struct {
	httpClient *http.Client
}

// NewBigScreenClient creates a new BigScreen client.
func NewBigScreenClient() *BigScreenClient {
	return &BigScreenClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// SearchShowtimes finds movies showing near the given zip code.
func (c *BigScreenClient) SearchShowtimes(zip string, radius int, date string) ([]Movie, error) {
	// Look up the center point for this zip code.
	centerLat, centerLon, err := zipToLatLon(zip)
	if err != nil {
		return nil, err
	}

	// Find theaters within radius.
	var nearby []Theater
	for _, t := range KnownTheaters {
		dist := haversine(centerLat, centerLon, t.Lat, t.Lon)
		if dist <= float64(radius) {
			nearby = append(nearby, t)
		}
	}

	if len(nearby) == 0 {
		return nil, fmt.Errorf("no known theaters within %d miles of %s", radius, zip)
	}

	// Fetch showtimes for each theater concurrently (up to 5 at a time).
	type fetchResult struct {
		theater Theater
		movies  []parsedMovie
		err     error
	}

	results := make(chan fetchResult, len(nearby))
	sem := make(chan struct{}, 5) // concurrency limit

	var wg sync.WaitGroup
	for _, t := range nearby {
		wg.Add(1)
		go func(theater Theater) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			movies, err := c.fetchTheater(theater.ID, date)
			results <- fetchResult{theater: theater, movies: movies, err: err}
		}(t)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Merge results: group by movie title.
	movieMap := make(map[string]*Movie)
	var movieOrder []string

	for r := range results {
		if r.err != nil {
			// Skip theaters that fail; don't abort everything.
			continue
		}
		for _, pm := range r.movies {
			key := pm.Title
			m, exists := movieMap[key]
			if !exists {
				m = &Movie{
					Title:   pm.Title,
					Rating:  pm.Rating,
					Runtime: pm.Runtime,
				}
				movieMap[key] = m
				movieOrder = append(movieOrder, key)
			}
			m.Theaters = append(m.Theaters, TheaterShowtime{
				TheaterID: r.theater.ID,
				Name:      r.theater.Name,
				City:      r.theater.City,
				Showtimes: pm.Showtimes,
				Features:  pm.Features,
			})
		}
	}

	// Build sorted result.
	var movies []Movie
	sort.Strings(movieOrder)
	for _, key := range movieOrder {
		movies = append(movies, *movieMap[key])
	}

	return movies, nil
}

// parsedMovie is the raw parsed data from a single theater page.
type parsedMovie struct {
	Title     string
	Rating    string
	Runtime   string
	Showtimes []string
	Features  string
}

// fetchTheater fetches and parses the printable showtime page for one theater.
func (c *BigScreenClient) fetchTheater(theaterID int, date string) ([]parsedMovie, error) {
	url := fmt.Sprintf(
		"https://www.bigscreen.com/Marquee.php?theater=%d&view=sched&printable=1&showdate=%s",
		theaterID, date,
	)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch theater %d: %w", theaterID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("theater %d returned status %d", theaterID, resp.StatusCode)
	}

	return parseShowtimePage(resp.Body)
}

// parseShowtimePage parses the BigScreen printable HTML page.
// Structure:
//
//	<div class="infoitem">
//	  <div class="infoitem_data movie">
//	    <span class="movie_name">Title</span><br>
//	    <span class="notes">[ Rating ] Runtime</span>
//	  </div>
//	  <div class="infoitem_data showtimes">
//	    <span class="showtimes">1:30, 6:15, 8:30</span><br>
//	    <span class="notes">Stadium Seating; Digital Projection</span>
//	  </div>
//	</div>
func parseShowtimePage(r interface{ Read([]byte) (int, error) }) ([]parsedMovie, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var movies []parsedMovie

	// Walk the DOM looking for div.infoitem elements.
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "infoitem") && !hasClass(n, "infoheading") {
			pm := extractMovie(n)
			if pm.Title != "" && len(pm.Showtimes) > 0 {
				movies = append(movies, pm)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return movies, nil
}

// extractMovie extracts movie info from a single div.infoitem node.
func extractMovie(n *html.Node) parsedMovie {
	var pm parsedMovie

	// Find the movie_name span.
	movieNameSpan := findSpanByClass(n, "movie_name")
	if movieNameSpan != nil {
		pm.Title = textContent(movieNameSpan)
	}

	// Find the notes span inside the movie div (contains rating and runtime).
	movieDiv := findDivByClass(n, "movie")
	if movieDiv != nil {
		notesSpan := findSpanByClass(movieDiv, "notes")
		if notesSpan != nil {
			ratingRuntime := textContent(notesSpan)
			pm.Rating, pm.Runtime = parseRatingRuntime(ratingRuntime)
		}
	}

	// Find showtimes span.
	showtimesDiv := findDivByClasses(n, "infoitem_data", "showtimes")
	if showtimesDiv == nil {
		// Try finding any div that contains a span.showtimes
		showtimesDiv = findDivContainingSpanClass(n, "showtimes")
	}
	if showtimesDiv != nil {
		stSpan := findSpanByClass(showtimesDiv, "showtimes")
		if stSpan != nil {
			timesStr := textContent(stSpan)
			pm.Showtimes = parseShowtimesList(timesStr)
		}
		// Features are in the notes span inside the showtimes div.
		featSpan := findSpanByClass(showtimesDiv, "notes")
		if featSpan != nil {
			pm.Features = cleanFeatures(textContent(featSpan))
		}
	}

	return pm
}

// parseRatingRuntime parses "[ PG-13 ] 1:35" into rating and runtime.
func parseRatingRuntime(s string) (string, string) {
	s = strings.TrimSpace(s)
	// Pattern: [ Rating ] Runtime
	re := regexp.MustCompile(`\[\s*(.+?)\s*\]\s*(.+)`)
	matches := re.FindStringSubmatch(s)
	if len(matches) == 3 {
		return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[2])
	}
	return "", s
}

// parseShowtimesList splits "1:30, 6:15, 8:30" or "11:00a, 3:45" into a slice.
func parseShowtimesList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var times []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			times = append(times, t)
		}
	}
	return times
}

// cleanFeatures tidies up the features string.
func cleanFeatures(s string) string {
	s = strings.TrimSpace(s)
	// Remove "Inaccessible" as it's not useful info.
	parts := strings.Split(s, ";")
	var cleaned []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && !strings.EqualFold(p, "Inaccessible") {
			cleaned = append(cleaned, p)
		}
	}
	return strings.Join(cleaned, "; ")
}

// === HTML helpers ===

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func getClass(n *html.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			return attr.Val
		}
	}
	return ""
}

func findSpanByClass(n *html.Node, class string) *html.Node {
	if n.Type == html.ElementNode && n.Data == "span" && hasClass(n, class) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findSpanByClass(c, class); found != nil {
			return found
		}
	}
	return nil
}

func findDivByClass(n *html.Node, class string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "div" && hasClass(c, class) {
			return c
		}
		if found := findDivByClass(c, class); found != nil {
			return found
		}
	}
	return nil
}

func findDivByClasses(n *html.Node, classes ...string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "div" {
			cls := getClass(c)
			allMatch := true
			for _, want := range classes {
				if !strings.Contains(cls, want) {
					allMatch = false
					break
				}
			}
			if allMatch {
				return c
			}
		}
		if found := findDivByClasses(c, classes...); found != nil {
			return found
		}
	}
	return nil
}

func findDivContainingSpanClass(n *html.Node, spanClass string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "div" {
			if findSpanByClass(c, spanClass) != nil {
				return c
			}
		}
		if found := findDivContainingSpanClass(c, spanClass); found != nil {
			return found
		}
	}
	return nil
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return strings.TrimSpace(sb.String())
}

// === Geo helpers ===

// haversine returns the distance in miles between two lat/lon points.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMi = 3958.8
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMi * c
}

// zipToLatLon maps a zip code to approximate lat/lon.
// Covers Bay Area, Central Valley, and Sacramento zips.
var zipCoords = map[string][2]float64{
	// Berkeley / Albany
	"94701": {37.8680, -122.2690}, "94702": {37.8640, -122.2840},
	"94703": {37.8625, -122.2760}, "94704": {37.8688, -122.2579},
	"94705": {37.8590, -122.2390}, "94706": {37.8880, -122.2980},
	"94707": {37.8930, -122.2730}, "94708": {37.8920, -122.2620},
	"94709": {37.8790, -122.2640}, "94710": {37.8650, -122.3000},
	// Oakland
	"94601": {37.7760, -122.2160}, "94602": {37.7990, -122.2100},
	"94603": {37.7380, -122.1820}, "94604": {37.8050, -122.2700},
	"94605": {37.7600, -122.1600}, "94606": {37.7930, -122.2420},
	"94607": {37.8060, -122.2860}, "94608": {37.8350, -122.2830},
	"94609": {37.8360, -122.2630}, "94610": {37.8120, -122.2380},
	"94611": {37.8290, -122.2200}, "94612": {37.8050, -122.2700},
	"94613": {37.7830, -122.1890}, "94618": {37.8440, -122.2410},
	"94619": {37.7830, -122.1950}, "94621": {37.7540, -122.2020},
	// Emeryville
	"94608a": {37.8310, -122.2840}, // covered by 94608
	// Alameda
	"94501": {37.7650, -122.2420}, "94502": {37.7350, -122.2420},
	// El Cerrito / Richmond
	"94530": {37.9160, -122.3100}, "94801": {37.9380, -122.3580},
	"94804": {37.9220, -122.3500}, "94805": {37.9340, -122.3330},
	// Walnut Creek
	"94595": {37.8850, -122.0600}, "94596": {37.9000, -122.0650},
	"94597": {37.9100, -122.0500}, "94598": {37.9050, -122.0400},
	// Concord
	"94518": {37.9770, -122.0310}, "94519": {37.9680, -122.0010},
	"94520": {37.9780, -122.0310}, "94521": {37.9630, -121.9760},
	// Pleasant Hill
	"94523": {37.9530, -122.0600},
	// Lafayette
	"94549": {37.8860, -122.1230},
	// Orinda
	"94563": {37.8770, -122.1830},
	// Moraga
	"94556": {37.8350, -122.1300},
	// Danville / San Ramon
	"94506": {37.8070, -121.9150}, "94526": {37.8210, -121.9680},
	"94583": {37.7620, -121.9500},
	// Martinez
	"94553": {38.0020, -122.1340},
	// Dublin / Pleasanton
	"94568": {37.7070, -121.9310}, "94588": {37.6960, -121.9260},
	"94566": {37.6600, -121.8750},
	// Livermore
	"94550": {37.6820, -121.7680}, "94551": {37.6930, -121.7130},
	// Fremont
	"94536": {37.5600, -121.9800}, "94538": {37.5370, -121.9820},
	"94539": {37.5190, -121.9420}, "94555": {37.5530, -122.0470},
	// Hayward
	"94541": {37.6690, -122.0810}, "94542": {37.6520, -122.0470},
	"94544": {37.6340, -122.0490}, "94545": {37.6300, -122.1000},
	// Castro Valley
	"94546": {37.6940, -122.0830}, "94552": {37.7050, -122.0580},
	// Union City / Newark
	"94587": {37.5930, -122.0180}, "94560": {37.5300, -122.0400},
	// San Leandro
	"94577": {37.7250, -122.1570}, "94578": {37.7080, -122.1270},
	"94579": {37.6970, -122.1430},
	// Larkspur / Mill Valley / Tiburon / Novato (Marin)
	"94939": {37.9340, -122.5350}, "94941": {37.9060, -122.5460},
	"94920": {37.8730, -122.4560}, "94947": {38.1010, -122.5570},
	"94949": {38.0870, -122.5550},
	// Stockton
	"95204": {37.9540, -121.3000}, "95207": {37.9780, -121.3110},
	"95209": {37.9930, -121.3280}, "95210": {38.0100, -121.2970},
	// Tracy
	"95376": {37.7350, -121.4260}, "95377": {37.7240, -121.4400},
	// Manteca
	"95336": {37.8050, -121.2200}, "95337": {37.7900, -121.2350},
	// Lodi
	"95240": {38.1300, -121.2720}, "95242": {38.1200, -121.2900},
	// Modesto
	"95350": {37.6370, -120.9940}, "95354": {37.6390, -120.9960},
	"95355": {37.6550, -121.0050}, "95356": {37.6610, -120.9730},
	// Sacramento
	"95811": {38.5720, -121.4930}, "95814": {38.5775, -121.4920},
	"95816": {38.5680, -121.4720}, "95818": {38.5570, -121.4900},
	"95819": {38.5630, -121.4580}, "95820": {38.5370, -121.4710},
	"95822": {38.5200, -121.4940}, "95825": {38.5900, -121.4200},
	"95834": {38.6380, -121.5010},
}

func zipToLatLon(zip string) (float64, float64, error) {
	coords, ok := zipCoords[zip]
	if !ok {
		return 0, 0, fmt.Errorf("unknown zip code: %s (add it to the zip database or use a known Bay Area / Central Valley zip)", zip)
	}
	return coords[0], coords[1], nil
}
