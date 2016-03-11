package geojson

type Feature struct {
	Type       string            `json:"type"`
	Features   []*Feature        `json:"features,omitempty"`
	Geometry   *Geometry         `json:"geometry,omitempty"`
	Id         *int64            `json:"id,omitempty"`
	Properties map[string]string `json:"properties"`
}

type Geometry struct {
	Type        string      `json:"type"`
	Coordinates interface{} `json:"coordinates"`
}

type Coordinate [2]float64
