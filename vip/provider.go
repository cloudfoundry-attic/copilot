package vip

type Provider struct {
}

func (Provider) Get(hostname string) string {
	return "127.0.0.1"
}
