package domain

// Movie representa el resultado puro de la búsqueda
type Movie struct {
	Title    string
	Year     string
	Rating   string
	Quality  string
	Language string
	URL      string
	Provider string
}

// StreamOption representa un enlace de video resuelto por un proveedor
type StreamOption struct {
	Server  string
	Quality string
	URL     string
}
