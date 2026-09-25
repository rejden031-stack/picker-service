package rest

import (
	"net"
	"net/http"
	"strings"
)

// IPResolver возвращает реальный IP клиента.
//
// Заголовкам X-Forwarded-For / X-Real-IP доверяем ТОЛЬКО когда peer-соединение
// пришло с доверенного прокси (ingress/LB). Иначе клиент подделает заголовок
// и обойдёт rate-limit.
type IPResolver func(*http.Request) string

// DefaultIPResolver — без доверенных прокси: берём только адрес соединения.
func DefaultIPResolver() IPResolver {
	return func(r *http.Request) string {
		return peerIP(r)
	}
}

// NewIPResolver строит резолвер, который верит прокси-заголовкам только от
// перечисленных сетей (CIDR или одиночный IP). Пустой список = не верить никому.
func NewIPResolver(trusted []string) IPResolver {
	trustedNets := make([]*net.IPNet, 0, len(trusted))
	for _, entry := range trusted {
		if ip := net.ParseIP(entry); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			trustedNets = append(trustedNets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		if _, cidr, err := net.ParseCIDR(entry); err == nil {
			trustedNets = append(trustedNets, cidr)
		}
	}

	return func(r *http.Request) string {
		peer := peerIP(r)
		if len(trustedNets) > 0 && peerIn(peer, trustedNets) {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
					return first
				}
			}
			if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
				return xrip
			}
		}
		return peer
	}
}

func peerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func peerIn(peer string, nets []*net.IPNet) bool {
	ip := net.ParseIP(peer)
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
