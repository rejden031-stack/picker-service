// Package audit — факты для журнала аудита: кто, что и когда сделал.
package audit

import "time"

// Action — тип события.
type Action string

const (
	LoginOK          Action = "login_ok"
	LoginFailed      Action = "login_failed"
	LoginBlocked     Action = "login_blocked"
	UserCreated      Action = "user_created"
	UserRoleChanged  Action = "user_role_changed"
	UserBlocked      Action = "user_blocked"
	UserUnblocked    Action = "user_unblocked"
	PasswordChanged  Action = "password_changed"
	ProductViewed    Action = "product_viewed"
	CandidatesViewed Action = "candidates_viewed"
)

// Entry — одна запись журнала.
type Entry struct {
	ID        int64
	Time      time.Time
	ActorID   int64
	ActorName string
	Action    Action
	EntityID  int64
	Details   string // компактная нотация: например "username=alice", "cell=12,sku=SKU-1"
}
