package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const reservationTTL = 15 * time.Minute

type reserveRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

type product struct {
	Inventory int
}

type reservation struct {
	ProductID string
	Quantity  int
	ExpiresAt time.Time
}

type reservationEngine struct {
	mu           sync.Mutex
	products     map[string]product
	reservations map[string]reservation
	now          func() time.Time
	newID        func() string
}

func newReservationEngine() *reservationEngine {
	return &reservationEngine{
		products: map[string]product{
			"field-notes": {Inventory: 42},
			"desk-lamp":   {Inventory: 11},
			"travel-mug":  {Inventory: 28},
		},
		reservations: make(map[string]reservation),
		now:          func() time.Time { return time.Now().UTC() },
		newID:        reservationID,
	}
}

func reservationID() string {
	bytes := make([]byte, 5)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("res_%d", time.Now().UnixNano())
	}
	return "res_" + hex.EncodeToString(bytes)
}

func stockConflict(c *gin.Context, available int) {
	c.JSON(http.StatusConflict, gin.H{
		"error":              "insufficient inventory",
		"suggested_quantity": available,
	})
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func firstEnv(fallback string, names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return fallback
}

func shortCommit() string {
	environment := env("VERCEL_ENV", "local")
	fallback := "CLI deployment"
	if environment == "local" || environment == "development" {
		fallback = "local"
	}
	commit := firstEnv(fallback, "VERCEL_GIT_COMMIT_SHA")
	if commit != fallback && len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

func serviceIdentity() gin.H {
	environment := env("VERCEL_ENV", "local")
	fallback := "CLI deployment"
	if environment == "local" || environment == "development" {
		fallback = "local"
	}
	return gin.H{
		"service":     "reservations",
		"runtime":     "Go + Gin reservation container",
		"environment": environment,
		"branch":      firstEnv(fallback, "VERCEL_GIT_COMMIT_REF"),
		"commit":      shortCommit(),
	}
}

func (engine *reservationEngine) releaseExpired(now time.Time) {
	for id, held := range engine.reservations {
		if now.Before(held.ExpiresAt) {
			continue
		}

		item := engine.products[held.ProductID]
		item.Inventory += held.Quantity
		engine.products[held.ProductID] = item
		delete(engine.reservations, id)
	}
}

func (engine *reservationEngine) reserve(c *gin.Context) {
	var request reserveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid reservation request"})
		return
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()

	now := engine.now()
	engine.releaseExpired(now)

	item, exists := engine.products[request.ProductID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	if request.Quantity > item.Inventory {
		stockConflict(c, item.Inventory)
		return
	}

	item.Inventory -= request.Quantity
	engine.products[request.ProductID] = item

	reservationID := engine.newID()
	expiresAt := now.Add(reservationTTL)
	engine.reservations[reservationID] = reservation{
		ProductID: request.ProductID,
		Quantity:  request.Quantity,
		ExpiresAt: expiresAt,
	}

	c.JSON(http.StatusCreated, gin.H{
		"reservation_id":          reservationID,
		"reservation_status":      "reserved",
		"reserved_until":          expiresAt.Format(time.RFC3339),
		"reservation_ttl_seconds": int(reservationTTL.Seconds()),
		"product_id":              request.ProductID,
		"quantity":                request.Quantity,
		"service":                 serviceIdentity(),
	})
}

func newRouter(engine *reservationEngine) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.POST("/internal/reserve", engine.reserve)
	router.GET("/internal/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": serviceIdentity()})
	})
	return router
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := newRouter(newReservationEngine())
	port := env("PORT", "8081")
	if err := router.Run("0.0.0.0:" + port); err != nil {
		panic(fmt.Sprintf("reservation server failed: %v", err))
	}
}
