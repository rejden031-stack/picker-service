package rest

import (
	"context"
	"log/slog"
	"net/http"

	authapp "picker-service/internal/application/auth"
	catalogapp "picker-service/internal/application/catalog"
	"picker-service/internal/application/locations"
	authports "picker-service/internal/application/ports/auth"
	"picker-service/internal/application/useradmin"
)

// Handler — «официант»: переводит HTTP в вызовы сценариев.
type Handler struct {
	login          *authapp.Login
	tree           *locations.Tree
	listProducts   *catalogapp.ListCellProducts
	findCandidates *catalogapp.FindCandidates
	tokenParser    authports.TokenParser
	userAdmin      *useradmin.UserAdmin
	changePassword *authapp.ChangePassword

	// ReadinessCheck — проверка зависимостей для /readyz. nil означает «всегда готов».
	ReadinessCheck func(context.Context) error
}

func NewHandler(login *authapp.Login, tree *locations.Tree,
	listProducts *catalogapp.ListCellProducts, findCandidates *catalogapp.FindCandidates,
	tokenParser authports.TokenParser, userAdmin *useradmin.UserAdmin,
	changePassword *authapp.ChangePassword) *Handler {
	return &Handler{
		login:          login,
		tree:           tree,
		listProducts:   listProducts,
		findCandidates: findCandidates,
		tokenParser:    tokenParser,
		userAdmin:      userAdmin,
		changePassword: changePassword,
	}
}

// Ready — readiness-проба: готов ли сервис принимать трафик.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.ReadinessCheck == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	if err := h.ReadinessCheck(r.Context()); err != nil {
		slog.Error("readiness check failed", "error", err)
		writeErr(w, http.StatusServiceUnavailable, "not ready")
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
