package service

import (
	"strings"

	"net/netip"
)

func lessIPString(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	ap, aok := netip.ParseAddr(a)
	bp, bok := netip.ParseAddr(b)
	if aok == nil && bok == nil {
		if ap.Is4() != bp.Is4() {
			return ap.Is4()
		}
		if ap != bp {
			return ap.Less(bp)
		}
		return a < b
	}
	if aok == nil {
		return true
	}
	if bok == nil {
		return false
	}
	return a < b
}
