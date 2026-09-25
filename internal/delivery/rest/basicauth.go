package rest

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

// basicAuth возвращает middleware, требующий HTTP Basic Auth.
// user == "" означает «не включать проверку».
func basicAuth(user, pass string) func(http.Handler) http.Handler {
	if user == "" {
		return func(next http.Handler) http.Handler { return next }
	}

	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := r.Header.Get("Authorization")
			if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
				writeErr(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
