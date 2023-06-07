package network

type Bridge struct{}

func (b *Bridge) Create(cidr, name string) (string, error) {
	return "", nil
}
