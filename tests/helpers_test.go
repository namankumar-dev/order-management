package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"order-management/apperror"
	"order-management/auth"
	"order-management/client"
	"order-management/config"
	"order-management/graph"
	"order-management/model"
	"order-management/router"
	"order-management/service"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type fakeOrderRepo struct {
	mu     sync.Mutex
	orders map[uint]*model.Order
	next   uint
}

func newFakeOrderRepo() *fakeOrderRepo {
	return &fakeOrderRepo{orders: map[uint]*model.Order{}}
}

func (r *fakeOrderRepo) Create(_ context.Context, o *model.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	o.ID = r.next
	o.CreatedAt = time.Now()
	o.UpdatedAt = o.CreatedAt
	cp := *o
	r.orders[o.ID] = &cp
	return nil
}

func (r *fakeOrderRepo) GetByID(_ context.Context, id uint) (*model.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.orders[id]
	if !ok {
		return nil, apperror.NotFound("order not found")
	}
	cp := *o
	return &cp, nil
}

func (r *fakeOrderRepo) List(_ context.Context, q model.ListQuery, customerID *uint) ([]model.Order, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []model.Order
	for _, o := range r.orders {
		if customerID != nil && o.CustomerID != *customerID {
			continue
		}
		if q.Status != "" && string(o.Status) != q.Status {
			continue
		}
		all = append(all, *o)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	total := int64(len(all))
	start := q.Offset()
	if start > len(all) {
		start = len(all)
	}
	end := start + q.PageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], total, nil
}

func (r *fakeOrderRepo) UpdateStatus(_ context.Context, id uint, apply func(*model.Order) error) (*model.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.orders[id]
	if !ok {
		return nil, apperror.NotFound("order not found")
	}
	cp := *stored
	if err := apply(&cp); err != nil {
		return nil, err
	}
	cp.UpdatedAt = time.Now()
	r.orders[id] = &cp
	out := cp
	return &out, nil
}

func (r *fakeOrderRepo) Ping(context.Context) error { return nil }

func (r *fakeOrderRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.orders)
}

type fakeUserRepo struct {
	mu    sync.Mutex
	users map[string]*model.User
	next  uint
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[string]*model.User{}}
}

func (r *fakeUserRepo) Create(_ context.Context, u *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[u.Email]; exists {
		return apperror.Conflict("email already registered")
	}
	r.next++
	u.ID = r.next
	u.CreatedAt = time.Now()
	cp := *u
	r.users[u.Email] = &cp
	return nil
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, email string) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[email]
	if !ok {
		return nil, apperror.NotFound("user not found")
	}
	cp := *u
	return &cp, nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id uint) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.ID == id {
			cp := *u
			return &cp, nil
		}
	}
	return nil, apperror.NotFound("user not found")
}

type fakeProducts struct {
	products map[uint]*client.Product
	err      error
}

func newFakeProducts() *fakeProducts {
	return &fakeProducts{products: map[uint]*client.Product{
		501: {ID: 501, Name: "Mechanical Keyboard", Price: 79.99, Stock: 100000},
		502: {ID: 502, Name: "USB-C Hub", Price: 34.5, Stock: 3},
	}}
}

func (f *fakeProducts) GetProduct(_ context.Context, id uint) (*client.Product, error) {
	if f.err != nil {
		return nil, f.err
	}
	p, ok := f.products[id]
	if !ok {
		return nil, client.ErrProductNotFound
	}
	cp := *p
	return &cp, nil
}

type testEnv struct {
	engine   *gin.Engine
	orders   *fakeOrderRepo
	products *fakeProducts
	orderSvc service.OrderService
	authSvc  service.AuthService
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	cfg := &config.Config{AppEnv: "test", JWTSecret: strings.Repeat("k", 32), JWTTTL: time.Minute}
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	orderRepo := newFakeOrderRepo()
	userRepo := newFakeUserRepo()
	products := newFakeProducts()
	orderSvc := service.NewOrderService(orderRepo, products)
	authSvc := service.NewAuthService(userRepo, tokens)
	schema, err := graph.NewSchema(orderSvc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	engine, err := router.New(router.Deps{
		Config: cfg,
		Orders: orderSvc,
		Auth:   authSvc,
		Tokens: tokens,
		Ready:  orderRepo.Ping,
		Schema: schema,
	})
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	return &testEnv{engine: engine, orders: orderRepo, products: products, orderSvc: orderSvc, authSvc: authSvc}
}

func (e *testEnv) serve(method, path, token string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			panic(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.engine.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
}

func (e *testEnv) login(t *testing.T, email, password string) string {
	t.Helper()
	rec := e.serve(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": email, "password": password})
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: status %d body %s", email, rec.Code, rec.Body.String())
	}
	var tok struct {
		AccessToken string `json:"accessToken"`
	}
	decode(t, rec, &tok)
	return tok.AccessToken
}

func (e *testEnv) customer(t *testing.T, email string) (string, uint) {
	t.Helper()
	password := "StrongPass123"
	rec := e.serve(http.MethodPost, "/api/v1/auth/register", "", map[string]string{"email": email, "password": password})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s: status %d body %s", email, rec.Code, rec.Body.String())
	}
	var u struct {
		ID uint `json:"id"`
	}
	decode(t, rec, &u)
	return e.login(t, email, password), u.ID
}

func (e *testEnv) admin(t *testing.T) string {
	t.Helper()
	if err := e.authSvc.SeedAdmin(context.Background(), "admin@example.com", "AdminPassw0rd!!"); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	return e.login(t, "admin@example.com", "AdminPassw0rd!!")
}

func orderBody(customerID uint, productID uint, quantity int) map[string]any {
	return map[string]any{"customerId": customerID, "productId": productID, "quantity": quantity}
}
