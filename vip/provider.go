package vip

import (
	"crypto/sha256"
	"net"
)

type Provider struct {
}

func (*Provider) Get(hostname string) string {
	hasher := sha256.New()
	hasher.Write([]byte(hostname))
	h := hasher.Sum(nil)

	// ensure last two bits to 1s so we never end in .1 or .0
	h[0] = h[0] | 0x03

	vip := net.IP{127, h[2], h[1], h[0]}
	return vip.String()
}
