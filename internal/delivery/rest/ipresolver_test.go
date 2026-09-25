package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// req с заданным peer и прокси-заголовками.
func ipReq(remoteAddr, xff, xrip string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = remoteAddr
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	if xrip != "" {
		r.Header.Set("X-Real-IP", xrip)
	}
	return r
}

func TestIPResolver_DefaultIgnoresForgedHeaders(t *testing.T) {
	resolve := DefaultIPResolver()

	// Клиент подделывает X-Forwarded-For — ему не верим, берём peer.
	ip := resolve(ipReq("203.0.113.7:51234", "1.2.3.4", "9.9.9.9"))
	if ip != "203.0.113.7" {
		t.Errorf("got %q, want peer 203.0.113.7", ip)
	}
}

func TestIPResolver_TrustsProxyOnlyForTrustedPeer(t *testing.T) {
	resolve := NewIPResolver([]string{"10.0.0.0/8", "127.0.0.1"})

	// Заголовок пришёл с доверенного прокси — берём первое значение из XFF.
	ip := resolve(ipReq("10.1.2.3:443", "203.0.113.7, 10.1.2.3", ""))
	if ip != "203.0.113.7" {
		t.Errorf("got %q, want 203.0.113.7 (from trusted proxy)", ip)
	}

	// Мимо доверенной сети (CVE-классика: подделка XFF) — игнорируем заголовок.
	ip = resolve(ipReq("203.0.113.7:51234", "1.2.3.4", ""))
	if ip != "203.0.113.7" {
		t.Errorf("got %q, want peer 203.0.113.7 (untrusted)", ip)
	}

	// IPv6-прокси из 127.0.0.1-эквивалента.
	ip = resolve(ipReq("[::1]:1234", "198.51.100.2", ""))
	if ip != "::1" {
		t.Errorf("got %q, want ::1 (::1 not in trusted list)", ip)
	}
}

func TestIPResolver_XRealIPFallback(t *testing.T) {
	resolve := NewIPResolver([]string{"172.16.0.0/12"})
	ip := resolve(ipReq("172.16.5.5:443", "", "198.51.100.9"))
	if ip != "198.51.100.9" {
		t.Errorf("got %q, want 198.51.100.9 (X-Real-IP)", ip)
	}
}
