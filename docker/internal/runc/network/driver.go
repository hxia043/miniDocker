package network

var drivers map[string]driver

type driver interface {
	Create(string, string) (string, error)
}
