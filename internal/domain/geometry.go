package domain

// Geometry 表示 GeoJSON 几何对象
type Geometry struct {
	Type        string `json:"type"`
	Coordinates []any  `json:"coordinates"`
}
