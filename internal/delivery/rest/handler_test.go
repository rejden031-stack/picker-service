package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"picker-service/internal/adapters/authimpl"
	adaptercache "picker-service/internal/adapters/cache"
	auditapp "picker-service/internal/application/audit"
	authapp "picker-service/internal/application/auth"
	catalogapp "picker-service/internal/application/catalog"
	"picker-service/internal/application/locations"
	"picker-service/internal/application/ports/store"
	"picker-service/internal/application/useradmin"
	"picker-service/internal/domain/audit"
	"picker-service/internal/domain/auth"
	"picker-service/internal/domain/catalog"
	"picker-service/internal/domain/location"
)

type httpUser struct {
	user    auth.User
	hash    string
	blocked bool
}

type httpUserStore struct {
	mu    sync.RWMutex
	users map[string]httpUser
	seq   int64
}

func (s *httpUserStore) UserByUsername(_ context.Context, username string) (store.Credentials, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok {
		return store.Credentials{}, auth.ErrNotFound
	}
	return store.Credentials{User: u.user, PasswordHash: u.hash, Blocked: u.blocked}, nil
}

func (s *httpUserStore) UserByID(_ context.Context, id int64) (auth.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.user.ID == id {
			return u.user, nil
		}
	}
	return auth.User{}, auth.ErrNotFound
}

func (s *httpUserStore) PasswordHashByID(_ context.Context, id int64) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.user.ID == id {
			return u.hash, nil
		}
	}
	return "", auth.ErrNotFound
}

func (s *httpUserStore) CreateUser(_ context.Context, username, passwordHash string, role auth.Role) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[username]; exists {
		return auth.User{}, auth.ErrUsernameTaken
	}
	s.seq++
	u := auth.User{ID: s.seq, Username: username, Role: role}
	s.users[username] = httpUser{user: u, hash: passwordHash}
	return u, nil
}

func (s *httpUserStore) ListUsers(_ context.Context) ([]auth.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]auth.User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u.user)
	}
	return users, nil
}

func (s *httpUserStore) SetRole(_ context.Context, id int64, role auth.Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, u := range s.users {
		if u.user.ID == id {
			u.user.Role = role
			s.users[name] = u
			return nil
		}
	}
	return auth.ErrNotFound
}

func (s *httpUserStore) SetBlocked(_ context.Context, id int64, blocked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, u := range s.users {
		if u.user.ID == id {
			u.blocked = blocked
			s.users[name] = u
			return nil
		}
	}
	return auth.ErrNotFound
}

func (s *httpUserStore) SetPasswordHash(_ context.Context, id int64, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, u := range s.users {
		if u.user.ID == id {
			u.hash = hash
			s.users[name] = u
			return nil
		}
	}
	return auth.ErrNotFound
}

type hashEq struct{}

func (hashEq) Hash(_ context.Context, password string) (string, error) { return password, nil }
func (hashEq) Verify(_ context.Context, hash, password string) bool    { return hash == password }

type httpAuditStore struct {
	mu      sync.Mutex
	entries []audit.Entry
}

func (s *httpAuditStore) Append(_ context.Context, e audit.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	return nil
}

// Post — реализация async-буфера; здесь пишем синхронно (тесты ждут согласованности).
func (s *httpAuditStore) Post(e audit.Entry) { _ = s.Append(context.Background(), e) }

func (s *httpAuditStore) List(_ context.Context, limit, offset int) ([]audit.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []audit.Entry
	for i := len(s.entries) - 1; i >= 0; i-- {
		if offset > 0 {
			offset--
			continue
		}
		if limit > 0 && len(out) >= limit {
			break
		}
		out = append(out, s.entries[i])
	}
	return out, nil
}

type httpCatalogStore struct {
	products   map[int64][]catalog.Product
	candidates map[int64][]catalog.Candidate
}

func (s *httpCatalogStore) ProductsByCell(_ context.Context, cellID int64) ([]catalog.Product, error) {
	return s.products[cellID], nil
}

func (s *httpCatalogStore) Candidates(_ context.Context, productID, _ int64) ([]catalog.Candidate, error) {
	return s.candidates[productID], nil
}

func (s *httpCatalogStore) Import(_ context.Context, _ []catalog.Product, _ []catalog.Placement) error {
	return nil
}

type httpTreeStore struct{}

func (httpTreeStore) Tree(_ context.Context) (location.Tree, error) { return location.Tree{}, nil }
func (httpTreeStore) ResolveCell(_ context.Context, _, _, _, _ string) (location.Cell, error) {
	return location.Cell{}, nil
}
func (httpTreeStore) CellAddress(_ context.Context, _ int64) (location.Address, error) {
	return location.Address{}, nil
}

var (
	_ store.UserStore      = (*httpUserStore)(nil)
	_ store.UserAdminStore = (*httpUserStore)(nil)
	_ store.AuditStore     = (*httpAuditStore)(nil)
	_ auditapp.Poster      = (*httpAuditStore)(nil)
	_ store.CatalogStore   = (*httpCatalogStore)(nil)
	_ store.LocationStore  = (*httpTreeStore)(nil)
)

func newTestRouter() *httptest.Server {
	return newTestRouterOpts()
}

func newTestRouterOpts(opts ...RouterOption) *httptest.Server {
	srv, _, _ := newTestFixture(opts...)
	return srv
}

func newTestFixture(opts ...RouterOption) (*httptest.Server, *httpUserStore, *httpAuditStore) {
	issuer := authimpl.NewTokenIssuer("test-secret")

	users := &httpUserStore{users: map[string]httpUser{
		"admin":  {user: auth.User{ID: 3, Username: "admin", Role: auth.RoleAdmin}, hash: "123456"},
		"senior": {user: auth.User{ID: 1, Username: "senior", Role: auth.RoleSenior}, hash: "123456"},
		"worker": {user: auth.User{ID: 2, Username: "worker", Role: auth.RoleWorker}, hash: "123456"},
		"fired":  {user: auth.User{ID: 4, Username: "fired", Role: auth.RoleWorker}, hash: "123456", blocked: true},
	}}
	auditStore := &httpAuditStore{}
	catalogStore := &httpCatalogStore{
		products: map[int64][]catalog.Product{
			1: {{ID: 1, SKU: "SKU-1001", Name: "Подшипник 6203", Barcode: "4600000000001"}},
		},
		candidates: map[int64][]catalog.Candidate{
			1: {{CellID: 7, Address: location.Address{FloorCode: "2", RackCode: "A", ShelfCode: "01", CellCode: "07"}, Qty: 3}},
		},
	}

	login := authapp.NewLogin(users, hashEq{}, issuer, auditStore)
	tree := locations.NewTree(httpTreeStore{})
	listProducts := catalogapp.NewListCellProducts(catalogStore, adaptercache.Noop{}, auditStore)
	findCandidates := catalogapp.NewFindCandidates(catalogStore, adaptercache.Noop{}, auditStore)
	userAdmin := useradmin.NewUserAdmin(users, auditStore, hashEq{})
	changePassword := authapp.NewChangePassword(users, hashEq{}, auditStore)

	h := NewHandler(login, tree, listProducts, findCandidates, issuer, userAdmin, changePassword)
	srv := httptest.NewServer(NewRouter(h, NewMetrics(nil), opts...))
	return srv, users, auditStore
}

func doReq(t *testing.T, method, url, token string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func loginToken(t *testing.T, base, username, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	resp := doReq(t, http.MethodPost, base+"/api/v1/auth/login", "", strings.NewReader(body))
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", resp.StatusCode, raw)
	}
	var lr loginResponse
	if err := json.Unmarshal([]byte(raw), &lr); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	return lr.Token
}

func TestHealth(t *testing.T) {
	srv := newTestRouter()
	defer srv.Close()

	resp := doReq(t, http.MethodGet, srv.URL+"/healthz", "", nil)
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusOK || raw != "ok" {
		t.Errorf("health = %d %q, want 200 ok", resp.StatusCode, raw)
	}
}

func TestLoginReturnsSenior(t *testing.T) {
	srv := newTestRouter()
	defer srv.Close()

	body := `{"username":"senior","password":"123456"}`
	resp := doReq(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "", strings.NewReader(body))
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, raw)
	}

	var lr loginResponse
	if err := json.Unmarshal([]byte(raw), &lr); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if lr.Token == "" {
		t.Error("token is empty")
	}
	if lr.Role != auth.RoleSenior {
		t.Errorf("role = %q, want %q", lr.Role, auth.RoleSenior)
	}
}

func TestLoginBlockedForbidden(t *testing.T) {
	srv := newTestRouter()
	defer srv.Close()

	body := `{"username":"fired","password":"123456"}`
	resp := doReq(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "", strings.NewReader(body))
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", resp.StatusCode, raw)
	}
}

func TestLoginRateLimit(t *testing.T) {
	srv := newTestRouterOpts(WithLoginRateLimit(1, 1))
	defer srv.Close()

	body := `{"username":"senior","password":"123456"}`
	first := doReq(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "", strings.NewReader(body))
	readBody(t, first)
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first login status = %d, want 200", first.StatusCode)
	}

	second := doReq(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "", strings.NewReader(body))
	raw := readBody(t, second)
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second login status = %d, want 429; body=%s", second.StatusCode, raw)
	}
}

func TestReadyWithoutCheck(t *testing.T) {
	srv := newTestRouter()
	defer srv.Close()

	resp := doReq(t, http.MethodGet, srv.URL+"/readyz", "", nil)
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusOK || raw != "ok" {
		t.Errorf("readyz = %d %q, want 200 ok", resp.StatusCode, raw)
	}
}

func TestTreeRequiresToken(t *testing.T) {
	srv := newTestRouter()
	defer srv.Close()

	resp := doReq(t, http.MethodGet, srv.URL+"/api/v1/locations/tree", "", nil)
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401; body=%s", resp.StatusCode, raw)
	}
}

func TestCandidatesForbiddenForWorker(t *testing.T) {
	srv := newTestRouter()
	defer srv.Close()

	token := loginToken(t, srv.URL, "worker", "123456")
	resp := doReq(t, http.MethodGet, srv.URL+"/api/v1/products/1/candidates", token, nil)
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403; body=%s", resp.StatusCode, raw)
	}
}

func TestCellProductsForSenior(t *testing.T) {
	srv := newTestRouter()
	defer srv.Close()

	token := loginToken(t, srv.URL, "senior", "123456")
	resp := doReq(t, http.MethodGet, srv.URL+"/api/v1/cells/1/products", token, nil)
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, raw)
	}
	if !strings.Contains(raw, "SKU-1001") {
		t.Errorf("body does not contain SKU-1001: %s", raw)
	}
}

func TestAdminListUsersForbidsSenior(t *testing.T) {
	srv := newTestRouter()
	defer srv.Close()

	token := loginToken(t, srv.URL, "senior", "123456")
	resp := doReq(t, http.MethodGet, srv.URL+"/api/v1/admin/users", token, nil)
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403; body=%s", resp.StatusCode, raw)
	}
}

func TestAdminCreateUserThenLogin(t *testing.T) {
	srv, _, _ := newTestFixture()
	defer srv.Close()

	adminTok := loginToken(t, srv.URL, "admin", "123456")
	body := `{"username":"newbie","password":"s3cret","role":"worker"}`
	resp := doReq(t, http.MethodPost, srv.URL+"/api/v1/admin/users", adminTok, strings.NewReader(body))
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.StatusCode, raw)
	}

	tok := loginToken(t, srv.URL, "newbie", "s3cret")
	if tok == "" {
		t.Error("new user could not login")
	}
}

func TestAdminConflicts(t *testing.T) {
	srv := newTestRouter()
	defer srv.Close()

	adminTok := loginToken(t, srv.URL, "admin", "123456")

	// дубль имени
	dup := doReq(t, http.MethodPost, srv.URL+"/api/v1/admin/users", adminTok,
		strings.NewReader(`{"username":"worker","password":"x","role":"worker"}`))
	readBody(t, dup)
	if dup.StatusCode != http.StatusConflict {
		t.Errorf("duplicate user status = %d, want 409", dup.StatusCode)
	}

	// снять себе роль админа нельзя
	demote := doReq(t, http.MethodPatch, srv.URL+"/api/v1/admin/users/3/role", adminTok,
		strings.NewReader(`{"role":"worker"}`))
	readBody(t, demote)
	if demote.StatusCode != http.StatusConflict {
		t.Errorf("demote self status = %d, want 409", demote.StatusCode)
	}

	// заблокировать себя нельзя
	lock := doReq(t, http.MethodPatch, srv.URL+"/api/v1/admin/users/3/blocked", adminTok,
		strings.NewReader(`{"blocked":true}`))
	readBody(t, lock)
	if lock.StatusCode != http.StatusConflict {
		t.Errorf("self-lock status = %d, want 409", lock.StatusCode)
	}
}

func TestAdminBlockAndChangePassword(t *testing.T) {
	srv, _, _ := newTestFixture()
	defer srv.Close()

	adminTok := loginToken(t, srv.URL, "admin", "123456")

	// блокируем worker (ID 2)
	block := doReq(t, http.MethodPatch, srv.URL+"/api/v1/admin/users/2/blocked", adminTok,
		strings.NewReader(`{"blocked":true}`))
	readBody(t, block)
	if block.StatusCode != http.StatusNoContent {
		t.Fatalf("block status = %d, want 204", block.StatusCode)
	}

	// залогиниться больше нельзя
	resp := doReq(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "",
		strings.NewReader(`{"username":"worker","password":"123456"}`))
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("blocked login status = %d, want 403; body=%s", resp.StatusCode, raw)
	}

	// смена пароля senior себе: старый неверный => 400
	senTok := loginToken(t, srv.URL, "senior", "123456")
	bad := doReq(t, http.MethodPost, srv.URL+"/api/v1/auth/change-password", senTok,
		strings.NewReader(`{"old_password":"nope","new_password":"verylong1"}`))
	readBody(t, bad)
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("wrong old pass status = %d, want 400", bad.StatusCode)
	}

	// корректная смена: с нового пароля можно войти
	ok := doReq(t, http.MethodPost, srv.URL+"/api/v1/auth/change-password", senTok,
		strings.NewReader(`{"old_password":"123456","new_password":"verylong1"}`))
	readBody(t, ok)
	if ok.StatusCode != http.StatusNoContent {
		t.Fatalf("change status = %d, want 204", ok.StatusCode)
	}
	if tok := loginToken(t, srv.URL, "senior", "verylong1"); tok == "" {
		t.Error("new password did not work")
	}
}

func TestAdminAuditRecordsLogin(t *testing.T) {
	srv, _, auditStore := newTestFixture()
	defer srv.Close()

	loginToken(t, srv.URL, "senior", "123456")

	adminTok := loginToken(t, srv.URL, "admin", "123456")
	resp := doReq(t, http.MethodGet, srv.URL+"/api/v1/admin/audit?limit=20", adminTok, nil)
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit status = %d, want 200; body=%s", resp.StatusCode, raw)
	}

	authCalls, _ := auditStore.List(context.Background(), 100, 0)
	if len(authCalls) == 0 {
		t.Fatal("audit is empty after logins")
	}
	hasLoginOK := false
	for _, e := range authCalls {
		if e.Action == audit.LoginOK && e.ActorName == "senior" {
			hasLoginOK = true
			break
		}
	}
	if !hasLoginOK {
		t.Error("audit does not contain login_ok for senior")
	}
}
