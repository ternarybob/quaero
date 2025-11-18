package places

// PlacesTextSearchResponse represents the Google Places Text Search API response
type PlacesTextSearchResponse struct {
	HTMLAttributions []string      `json:"html_attributions"`
	Results          []PlaceResult `json:"results"`
	Status           string        `json:"status"`
	ErrorMessage     string        `json:"error_message,omitempty"`
	NextPageToken    string        `json:"next_page_token,omitempty"`
}

// PlacesNearbySearchResponse represents the Google Places Nearby Search API response
type PlacesNearbySearchResponse struct {
	HTMLAttributions []string      `json:"html_attributions"`
	Results          []PlaceResult `json:"results"`
	Status           string        `json:"status"`
	ErrorMessage     string        `json:"error_message,omitempty"`
	NextPageToken    string        `json:"next_page_token,omitempty"`
}

// PlaceResult represents a single place result from Google Places API
type PlaceResult struct {
	BusinessStatus      string        `json:"business_status,omitempty"`
	FormattedAddress    string        `json:"formatted_address,omitempty"`
	Geometry            *Geometry     `json:"geometry,omitempty"`
	Icon                string        `json:"icon,omitempty"`
	IconBackgroundColor string        `json:"icon_background_color,omitempty"`
	IconMaskBaseURI     string        `json:"icon_mask_base_uri,omitempty"`
	Name                string        `json:"name"`
	OpeningHours        *OpeningHours `json:"opening_hours,omitempty"`
	Photos              []Photo       `json:"photos,omitempty"`
	PlaceID             string        `json:"place_id"`
	PlusCode            *PlusCode     `json:"plus_code,omitempty"`
	PriceLevel          int           `json:"price_level,omitempty"`
	Rating              float64       `json:"rating,omitempty"`
	Reference           string        `json:"reference,omitempty"`
	Types               []string      `json:"types,omitempty"`
	UserRatingsTotal    int           `json:"user_ratings_total,omitempty"`
	Vicinity            string        `json:"vicinity,omitempty"`
}

// Geometry represents the geometry information of a place
type Geometry struct {
	Location *LatLng `json:"location,omitempty"`
	Viewport *Bounds `json:"viewport,omitempty"`
}

// LatLng represents a geographic coordinate
type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Bounds represents a geographic bounding box
type Bounds struct {
	Northeast *LatLng `json:"northeast,omitempty"`
	Southwest *LatLng `json:"southwest,omitempty"`
}

// OpeningHours represents the opening hours of a place
type OpeningHours struct {
	OpenNow     bool     `json:"open_now,omitempty"`
	Periods     []Period `json:"periods,omitempty"`
	WeekdayText []string `json:"weekday_text,omitempty"`
}

// Period represents a single opening period
type Period struct {
	Open  *DayTime `json:"open,omitempty"`
	Close *DayTime `json:"close,omitempty"`
}

// DayTime represents a specific day and time
type DayTime struct {
	Day  int    `json:"day"`
	Time string `json:"time"`
}

// Photo represents a place photo reference
type Photo struct {
	Height           int      `json:"height"`
	HTMLAttributions []string `json:"html_attributions"`
	PhotoReference   string   `json:"photo_reference"`
	Width            int      `json:"width"`
}

// PlusCode represents a plus code (Open Location Code)
type PlusCode struct {
	CompoundCode string `json:"compound_code,omitempty"`
	GlobalCode   string `json:"global_code,omitempty"`
}
