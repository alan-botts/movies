package showtimes

// Theater represents a known theater with its BigScreen ID and location.
type Theater struct {
	ID   int
	Name string
	City string
	Lat  float64
	Lon  float64
}

// KnownTheaters is a hardcoded database of theaters we track.
// IDs come from BigScreen Cinema Guide (bigscreen.com).
var KnownTheaters = []Theater{
	// === Berkeley / Oakland / Emeryville / Alameda ===
	{ID: 1203, Name: "Rialto Cinemas Elmwood", City: "Berkeley", Lat: 37.8660, Lon: -122.2595},
	{ID: 2111, Name: "Pacific Film Archive", City: "Berkeley", Lat: 37.8690, Lon: -122.2660},
	{ID: 1200, Name: "Regal Jack London", City: "Oakland", Lat: 37.7955, Lon: -122.2778},
	{ID: 1205, Name: "Grand Lake Theater", City: "Oakland", Lat: 37.8122, Lon: -122.2517},
	{ID: 1231, Name: "Landmark Piedmont Theatre", City: "Oakland", Lat: 37.8270, Lon: -122.2537},
	{ID: 31455, Name: "The New Parkway Theater", City: "Oakland", Lat: 37.8100, Lon: -122.2700},
	{ID: 8329, Name: "AMC Bay Street 16", City: "Emeryville", Lat: 37.8320, Lon: -122.2930},
	{ID: 10922, Name: "Alameda Theatre & Cineplex", City: "Alameda", Lat: 37.7652, Lon: -122.2444},

	// === Walnut Creek / Concord / Pleasant Hill / Lafayette / Orinda / Moraga ===
	{ID: 8558, Name: "Cinemark Walnut Creek 14 and XD", City: "Walnut Creek", Lat: 37.9060, Lon: -122.0650},
	{ID: 3577, Name: "Brenden Theatres Concord 14", City: "Concord", Lat: 37.9780, Lon: -122.0310},
	{ID: 7977, Name: "Century Pleasant Hill 16", City: "Pleasant Hill", Lat: 37.9530, Lon: -122.0600},
	{ID: 1207, Name: "Orinda Theatre", City: "Orinda", Lat: 37.8770, Lon: -122.1830},
	{ID: 1629, Name: "Rheem Theatre", City: "Moraga", Lat: 37.8515, Lon: -122.1285},
	{ID: 1239, Name: "Contra Costa Stadium Cinemas", City: "Martinez", Lat: 37.9980, Lon: -122.1340},
	{ID: 1218, Name: "Apple Cinemas Blackhawk Plaza", City: "Danville", Lat: 37.8060, Lon: -121.9170},

	// === Dublin / Pleasanton / Livermore ===
	{ID: 7207, Name: "Regal Hacienda Crossings 20 & IMAX", City: "Dublin", Lat: 37.7070, Lon: -121.9310},
	{ID: 101164, Name: "The LOT City Center", City: "Dublin", Lat: 37.7035, Lon: -121.9380},
	{ID: 1610, Name: "Vine Cinema & Alehouse", City: "Livermore", Lat: 37.6818, Lon: -121.7690},
	{ID: 10362, Name: "Livermore Cinemas", City: "Livermore", Lat: 37.6815, Lon: -121.7685},

	// === Fremont / Hayward / Castro Valley / Union City ===
	{ID: 23441, Name: "Cine Lounge Fremont 7", City: "Fremont", Lat: 37.5485, Lon: -121.9886},
	{ID: 29354, Name: "Century at Pacific Commons", City: "Fremont", Lat: 37.5090, Lon: -121.9610},
	{ID: 42555, Name: "AMC NewPark 12", City: "Newark", Lat: 37.5300, Lon: -122.0130},
	{ID: 5994, Name: "Century 25 Union Landing and XD", City: "Union City", Lat: 37.5920, Lon: -122.0180},
	{ID: 11586, Name: "Century at Hayward", City: "Hayward", Lat: 37.6710, Lon: -122.0810},
	{ID: 7779, Name: "Century 16 Bayfair Mall", City: "San Leandro", Lat: 37.6930, Lon: -122.1290},
	{ID: 7376, Name: "Bal Theatre", City: "San Leandro", Lat: 37.7250, Lon: -122.1570},
	{ID: 1036, Name: "Chabot Cinema", City: "Castro Valley", Lat: 37.6945, Lon: -122.0835},
	{ID: 100751, Name: "Veranda LUXE Cinema & IMAX", City: "Concord", Lat: 37.9560, Lon: -122.0560},

	// === Richmond / El Cerrito / San Pablo ===
	{ID: 10342, Name: "Rialto Cinemas Cerrito", City: "El Cerrito", Lat: 37.9180, Lon: -122.3100},
	{ID: 7971, Name: "Century Richmond Hilltop 16", City: "Richmond", Lat: 37.9510, Lon: -122.3590},

	// === Marin County ===
	{ID: 1202, Name: "Lark Theater", City: "Larkspur", Lat: 37.9340, Lon: -122.5350},
	{ID: 1238, Name: "Larkspur Landing Cinema", City: "Larkspur", Lat: 37.9460, Lon: -122.5150},
	{ID: 1221, Name: "Cinelounge Tiburon", City: "Tiburon", Lat: 37.8730, Lon: -122.4560},
	{ID: 1217, Name: "Sequoia Cinema", City: "Mill Valley", Lat: 37.9060, Lon: -122.5460},
	{ID: 1214, Name: "Century Rowland Plaza", City: "Novato", Lat: 38.1010, Lon: -122.5570},

	// === Stockton / Tracy / Modesto / Sacramento ===
	{ID: 2297, Name: "Regal Stockton Holiday", City: "Stockton", Lat: 37.9780, Lon: -121.3110},
	{ID: 8671, Name: "Regal Stockton City Center 16", City: "Stockton", Lat: 37.9540, Lon: -121.2900},
	{ID: 7902, Name: "Lodi Stadium 12 Cinemas", City: "Lodi", Lat: 38.1300, Lon: -121.2720},
	{ID: 2872, Name: "Cinemark Tracy Movies 14", City: "Tracy", Lat: 37.7350, Lon: -121.4260},
	{ID: 11750, Name: "AMC Manteca 16", City: "Manteca", Lat: 37.8050, Lon: -121.2200},
	{ID: 6063, Name: "Brenden Theatres Modesto 18", City: "Modesto", Lat: 37.6367, Lon: -120.9942},
	{ID: 5159, Name: "Regal Modesto Stadium 10", City: "Modesto", Lat: 37.6510, Lon: -121.0000},
	{ID: 3568, Name: "Crest Theatre", City: "Sacramento", Lat: 38.5775, Lon: -121.4920},
	{ID: 6533, Name: "Esquire IMAX Theatre", City: "Sacramento", Lat: 38.5635, Lon: -121.4930},
	{ID: 100850, Name: "Century DOCO and XD", City: "Sacramento", Lat: 38.5810, Lon: -121.4990},
	{ID: 3559, Name: "The Tower Theatre by Angelika", City: "Sacramento", Lat: 38.5585, Lon: -121.4680},

	// === Solano County ===
	{ID: 1604, Name: "West Wind Solano Drive-In", City: "Concord", Lat: 38.0050, Lon: -122.0290},
}
