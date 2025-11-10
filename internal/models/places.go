package models

// PlacesSearchRequest represents a request to search for places
type PlacesSearchRequest struct {
	SearchQuery string                 `json:"search_query"`
	SearchType  string                 `json:"search_type"` // text_search, nearby_search
	MaxResults  int                    `json:"max_results,omitempty"`
	ListName    string                 `json:"list_name,omitempty"` // Optional: override auto-generated name
	Filters     map[string]interface{} `json:"filters,omitempty"`
	Location    *Location              `json:"location,omitempty"` // For nearby_search
}

// Location represents geographic coordinates
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Radius    int     `json:"radius,omitempty"` // meters
}

// PlacesSearchResult represents the result of a places search (stored in job progress)
type PlacesSearchResult struct {
	SearchQuery  string      `json:"search_query"`
	SearchType   string      `json:"search_type"`
	TotalResults int         `json:"total_results"`
	Places       []PlaceItem `json:"places"`
}

// PlaceItem represents an individual place from Google Places API
type PlaceItem struct {
	PlaceID                  string  `json:"place_id"`
	Name                     string  `json:"name"`
	FormattedAddress         string  `json:"formatted_address,omitempty"`
	PhoneNumber              string  `json:"phone_number,omitempty"`
	InternationalPhoneNumber string  `json:"international_phone_number,omitempty"`
	Website                  string  `json:"website,omitempty"`
	Rating                   float64 `json:"rating,omitempty"`
	UserRatingsTotal         int     `json:"user_ratings_total,omitempty"`
	PriceLevel               int     `json:"price_level,omitempty"`
	Latitude                 float64 `json:"latitude,omitempty"`
	Longitude                float64 `json:"longitude,omitempty"`
	Types                    []string `json:"types,omitempty"`
	OpeningHours             interface{} `json:"opening_hours,omitempty"`
	Photos                   []interface{} `json:"photos,omitempty"`
}
