package auth

import "errors"

// ErrInvalidToken — пропуск не прошёл проверку подписи/срока.
var ErrInvalidToken = errors.New("invalid token")
