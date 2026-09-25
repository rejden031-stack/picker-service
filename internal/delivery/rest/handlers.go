package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	authapp "picker-service/internal/application/auth"
	"picker-service/internal/domain/audit"
	"picker-service/internal/domain/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string    `json:"token"`
	Role  auth.Role `json:"role"`
}

type productResponse struct {
	ID      int64  `json:"id"`
	SKU     string `json:"sku"`
	Name    string `json:"name"`
	Barcode string `json:"barcode"`
}

type candidateResponse struct {
	CellID    int64  `json:"cell_id"`
	Address   string `json:"address"`
	Qty       int    `json:"qty"`
	IsPrimary bool   `json:"is_primary"`
}

type createUserRequest struct {
	Username string    `json:"username"`
	Password string    `json:"password"`
	Role     auth.Role `json:"role"`
}

type roleRequest struct {
	Role auth.Role `json:"role"`
}

type blockedRequest struct {
	Blocked bool `json:"blocked"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type auditEntryResponse struct {
	ID        int64        `json:"id"`
	Time      string       `json:"time"`
	ActorID   int64        `json:"actor_id"`
	ActorName string       `json:"actor_name"`
	Action    audit.Action `json:"action"`
	EntityID  int64        `json:"entity_id"`
	Details   string       `json:"details"`
}

type userResponse struct {
	ID       int64     `json:"id"`
	Username string    `json:"username"`
	Role     auth.Role `json:"role"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}

	remoteIP, _ := r.Context().Value(ipKey).(string)
	token, role, err := h.login.Login(r.Context(), req.Username, req.Password, remoteIP)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrBlocked):
			writeErr(w, http.StatusForbidden, "account blocked")
		case errors.Is(err, authapp.ErrInvalidCredentials):
			writeErr(w, http.StatusUnauthorized, "invalid credentials")
		default:
			writeErr(w, http.StatusInternalServerError, "login failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{Token: token, Role: role})
}

func (h *Handler) Tree(w http.ResponseWriter, r *http.Request) {
	tree, err := h.tree.Get(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "tree failed")
		return
	}
	writeJSON(w, http.StatusOK, tree)
}

func (h *Handler) CellProducts(w http.ResponseWriter, r *http.Request) {
	cellID, err := strconv.ParseInt(chi.URLParam(r, "cellID"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad cell id")
		return
	}

	products, err := h.listProducts.List(r.Context(), userFrom(r), cellID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list products failed")
		return
	}

	resp := make([]productResponse, 0, len(products))
	for _, p := range products {
		resp = append(resp, productResponse{ID: p.ID, SKU: p.SKU, Name: p.Name, Barcode: p.Barcode})
	}
	writeJSON(w, http.StatusOK, map[string][]productResponse{"products": resp})
}

func (h *Handler) Candidates(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(chi.URLParam(r, "productID"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad product id")
		return
	}
	exclude, _ := strconv.ParseInt(r.URL.Query().Get("exclude_cell"), 10, 64)

	candidates, err := h.findCandidates.Find(r.Context(), userFrom(r), productID, exclude)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "candidates failed")
		return
	}

	resp := make([]candidateResponse, 0, len(candidates))
	for _, c := range candidates {
		resp = append(resp, candidateResponse{
			CellID:    c.CellID,
			Address:   c.Address.String(),
			Qty:       c.Qty,
			IsPrimary: c.IsPrimary,
		})
	}
	writeJSON(w, http.StatusOK, map[string][]candidateResponse{"candidates": resp})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}

	err := h.changePassword.Change(r.Context(), userFrom(r), req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrWeakPassword):
			writeErr(w, http.StatusBadRequest, "password too short")
		case errors.Is(err, auth.ErrWrongPassword):
			writeErr(w, http.StatusBadRequest, "wrong old password")
		default:
			writeErr(w, http.StatusInternalServerError, "change password failed")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userAdmin.ListUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list users failed")
		return
	}

	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, userResponse{ID: u.ID, Username: u.Username, Role: u.Role})
	}
	writeJSON(w, http.StatusOK, map[string][]userResponse{"users": resp})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}

	u, err := h.userAdmin.CreateUser(r.Context(), userFrom(r), req.Username, req.Password, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUsernameTaken):
			writeErr(w, http.StatusConflict, "username taken")
		case errors.Is(err, auth.ErrInvalidRole):
			writeErr(w, http.StatusBadRequest, "invalid role")
		default:
			writeErr(w, http.StatusInternalServerError, "create user failed")
		}
		return
	}
	writeJSON(w, http.StatusCreated, userResponse{ID: u.ID, Username: u.Username, Role: u.Role})
}

func (h *Handler) SetUserRole(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad user id")
		return
	}
	var req roleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}

	err = h.userAdmin.SetRole(r.Context(), userFrom(r), userID, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrNotFound):
			writeErr(w, http.StatusNotFound, "user not found")
		case errors.Is(err, auth.ErrInvalidRole):
			writeErr(w, http.StatusBadRequest, "invalid role")
		case errors.Is(err, auth.ErrLastAdmin):
			writeErr(w, http.StatusConflict, "cannot demote last admin")
		default:
			writeErr(w, http.StatusInternalServerError, "set role failed")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetUserBlocked(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad user id")
		return
	}
	var req blockedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}

	err = h.userAdmin.SetBlocked(r.Context(), userFrom(r), userID, req.Blocked)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrNotFound):
			writeErr(w, http.StatusNotFound, "user not found")
		case errors.Is(err, auth.ErrSelfLockout):
			writeErr(w, http.StatusConflict, "cannot block yourself")
		default:
			writeErr(w, http.StatusInternalServerError, "set blocked failed")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad user id")
		return
	}
	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}

	err = h.userAdmin.ResetPassword(r.Context(), userFrom(r), userID, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "user not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "reset password failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListAudit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	entries, err := h.userAdmin.ListAudit(r.Context(), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list audit failed")
		return
	}

	resp := make([]auditEntryResponse, 0, len(entries))
	for _, e := range entries {
		resp = append(resp, auditEntryResponse{
			ID:        e.ID,
			Time:      e.Time.UTC().Format("2006-01-02T15:04:05Z"),
			ActorID:   e.ActorID,
			ActorName: e.ActorName,
			Action:    e.Action,
			EntityID:  e.EntityID,
			Details:   e.Details,
		})
	}
	writeJSON(w, http.StatusOK, map[string][]auditEntryResponse{"entries": resp})
}
