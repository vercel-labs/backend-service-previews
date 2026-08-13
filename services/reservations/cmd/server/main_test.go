package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestShortCommitUsesLocalFallback(t *testing.T) {
	t.Setenv("VERCEL_GIT_COMMIT_SHA", "")
	if got := shortCommit(); got != "local" {
		t.Fatalf("shortCommit() = %q, want local", got)
	}
}

func TestIdentityUsesVercelGitSource(t *testing.T) {
	t.Setenv("VERCEL_ENV", "production")
	t.Setenv("VERCEL_GIT_COMMIT_REF", "release/stock-holds")
	t.Setenv("VERCEL_GIT_COMMIT_SHA", "abcdef1234567890")

	identity := serviceIdentity()
	if identity["branch"] != "release/stock-holds" {
		t.Fatalf("branch = %v", identity["branch"])
	}
	if identity["commit"] != "abcdef1" {
		t.Fatalf("commit = %v", identity["commit"])
	}
}

func TestCLIDeploymentReportsNeutralSource(t *testing.T) {
	t.Setenv("VERCEL_ENV", "production")
	t.Setenv("VERCEL_GIT_COMMIT_REF", "")
	t.Setenv("VERCEL_GIT_COMMIT_SHA", "")
	identity := serviceIdentity()
	if identity["branch"] != "CLI deployment" {
		t.Fatalf("branch = %v", identity["branch"])
	}
	if identity["commit"] != "CLI deployment" {
		t.Fatalf("commit = %v", identity["commit"])
	}
}

func TestReservationHoldsStockForFifteenMinutes(t *testing.T) {
	ginTestMode(t)
	now := time.Date(2026, time.August, 12, 10, 0, 0, 0, time.UTC)
	engine := newReservationEngine()
	engine.now = func() time.Time { return now }
	engine.newID = func() string { return "res_test" }

	response := reserveProduct(t, newRouter(engine), 2)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}

	var body struct {
		ReservationID string `json:"reservation_id"`
		ReservedUntil string `json:"reserved_until"`
		ProductID     string `json:"product_id"`
		Quantity      int    `json:"quantity"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ReservationID != "res_test" || body.ProductID != "field-notes" || body.Quantity != 2 {
		t.Fatalf("unexpected reservation: %+v", body)
	}
	if body.ReservedUntil != "2026-08-12T10:15:00Z" {
		t.Fatalf("reserved_until = %q", body.ReservedUntil)
	}
}

func TestOverInventoryReservationSuggestsRemainingStock(t *testing.T) {
	ginTestMode(t)
	engine := newReservationEngine()

	response := reserveProduct(t, newRouter(engine), 43)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}
	var body struct {
		SuggestedQuantity int `json:"suggested_quantity"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SuggestedQuantity != 42 {
		t.Fatalf("suggested_quantity = %d, want the 42 items still in stock", body.SuggestedQuantity)
	}
	if len(engine.reservations) != 0 || engine.products["field-notes"].Inventory != 42 {
		t.Fatalf("rejected request changed reservation state")
	}
}

func TestConcurrentReservationsCannotOversell(t *testing.T) {
	ginTestMode(t)
	engine := newReservationEngine()
	router := newRouter(engine)

	var wait sync.WaitGroup
	statuses := make(chan int, 43)
	for range 43 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			statuses <- reserveProduct(t, router, 1).Code
		}()
	}
	wait.Wait()
	close(statuses)

	created := 0
	conflicts := 0
	for status := range statuses {
		if status == http.StatusCreated {
			created++
		}
		if status == http.StatusConflict {
			conflicts++
		}
	}
	if created != 42 {
		t.Fatalf("created = %d, want 42", created)
	}
	if conflicts != 1 {
		t.Fatalf("conflicts = %d, want 1", conflicts)
	}
}

func TestExpiredReservationReturnsStock(t *testing.T) {
	ginTestMode(t)
	now := time.Date(2026, time.August, 12, 10, 0, 0, 0, time.UTC)
	engine := newReservationEngine()
	engine.now = func() time.Time { return now }
	engine.newID = func() string { return "res_first" }
	router := newRouter(engine)

	first := reserveProduct(t, router, 5)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d", first.Code)
	}

	now = now.Add(reservationTTL + time.Second)
	engine.newID = func() string { return "res_second" }
	second := reserveProduct(t, router, 5)
	if second.Code != http.StatusCreated {
		t.Fatalf("second status = %d, want expired stock to be available: %s", second.Code, second.Body.String())
	}
}

func reserveProduct(t *testing.T, handler http.Handler, quantity int) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"product_id": "field-notes",
		"quantity":   quantity,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/internal/reserve", bytes.NewReader(body))
	request.Header.Set("content-type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func ginTestMode(t *testing.T) {
	t.Helper()
	t.Setenv("GIN_MODE", "test")
}
