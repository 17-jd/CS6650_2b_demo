package main

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Product follows the Product schema in api.yaml. int32 also checks the upper limit.
// Weight is a pointer so we can tell missing weight apart from a valid zero weight.
type Product struct {
	ProductID    int32  `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int32  `json:"category_id"`
	Weight       *int32 `json:"weight"`
	SomeOtherID  int32  `json:"some_other_id"`
}

// Store owns the map and the lock that protects it from concurrent requests.
type Store struct {
	mu       sync.RWMutex
	products map[int32]Product
}

func newStore() *Store {
	weight := int32(1250)
	return &Store{products: map[int32]Product{
		1:     {1, "DEMO-001", "Demo Manufacturer", 456, &weight, 789},
		12345: {12345, "ABC-123-XYZ", "Acme Corporation", 456, &weight, 789},
	}}
}

func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code, "message": message})
}

func productID(c *gin.Context) (int32, bool) {
	id, err := strconv.ParseInt(c.Param("productId"), 10, 32)
	return int32(id), err == nil && id > 0
}

func (s *Store) getProduct(c *gin.Context) {
	id, valid := productID(c)
	if !valid {
		// GET has no 400 response in the supplied contract.
		fail(c, 404, "NOT_FOUND", "Product ID must be a positive int32")
		return
	}
	s.mu.RLock()
	product, found := s.products[id]
	s.mu.RUnlock()
	if !found {
		fail(c, 404, "NOT_FOUND", "Product not found")
		return
	}
	c.JSON(200, product)
}

func (s *Store) updateProduct(c *gin.Context) {
	id, valid := productID(c)
	if !valid {
		fail(c, 400, "INVALID_INPUT", "Product ID must be a positive int32")
		return
	}
	if c.ContentType() != "application/json" {
		fail(c, 400, "INVALID_INPUT", "Content-Type must be application/json")
		return
	}
	// Limit memory used by one request. Only one JSON object is accepted.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	decoder := json.NewDecoder(c.Request.Body)
	var product *Product
	if err := decoder.Decode(&product); err != nil || product == nil {
		fail(c, 400, "INVALID_INPUT", "Body must be a Product JSON object")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		fail(c, 400, "INVALID_INPUT", "Body must contain exactly one JSON object")
		return
	}
	if product.ProductID != id || product.CategoryID < 1 || product.SomeOtherID < 1 ||
		product.Weight == nil || *product.Weight < 0 ||
		utf8.RuneCountInString(product.SKU) < 1 || utf8.RuneCountInString(product.SKU) > 100 ||
		utf8.RuneCountInString(product.Manufacturer) < 1 || utf8.RuneCountInString(product.Manufacturer) > 200 {
		fail(c, 400, "INVALID_INPUT", "Check required fields, field limits, and matching product_id")
		return
	}
	s.mu.Lock()
	_, found := s.products[id]
	if found {
		s.products[id] = *product
	}
	s.mu.Unlock()
	if !found {
		fail(c, 404, "NOT_FOUND", "Product not found")
		return
	}
	c.Status(204) // Successful POST has no response body.
}

// This middleware records RED metrics and one JSON log for each API request.
func observe(registry *prometheus.Registry) gin.HandlerFunc {
	labels := []string{"method", "endpoint"}
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "product_requests_total", Help: "API requests"}, labels)
	errors := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "product_errors_total", Help: "Non-2xx API responses"}, labels)
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "product_request_duration_seconds", Help: "API latency in seconds", Buckets: prometheus.DefBuckets}, labels)
	registry.MustRegister(requests, errors, duration)
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" || c.Request.URL.Path == "/health" {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = "unmatched"
		} // Do not label by arbitrary user IDs.
		elapsed := time.Since(start).Seconds()
		method := c.Request.Method
		requests.WithLabelValues(method, endpoint).Inc()
		errors.WithLabelValues(method, endpoint).Add(0)
		if c.Writer.Status() < 200 || c.Writer.Status() >= 300 {
			errors.WithLabelValues(method, endpoint).Inc()
		}
		duration.WithLabelValues(method, endpoint).Observe(elapsed)
		entry, _ := json.Marshal(gin.H{"time": time.Now().UTC().Format(time.RFC3339Nano), "method": method, "endpoint": endpoint, "status": c.Writer.Status(), "duration_ms": elapsed * 1000})
		log.Print(string(entry))
	}
}

func newRouter(key string, testErrors bool) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	registry := prometheus.NewRegistry()
	r.Use(observe(registry))
	// Keep the recovery handler from dumping request headers (including API keys).
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) { fail(c, 500, "INTERNAL_ERROR", "Internal server error") }))
	r.GET("/health", func(c *gin.Context) { c.Status(200) })
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(registry, promhttp.HandlerOpts{})))
	store := newStore()
	// The spec's server URL includes /v1. Unprefixed paths also work for class demos.
	for _, prefix := range []string{"", "/v1"} {
		api := r.Group(prefix)
		api.Use(func(c *gin.Context) {
			// ApiKeyAuth is one of the two alternatives in api.yaml.
			if subtle.ConstantTimeCompare([]byte(c.GetHeader("X-API-Key")), []byte(key)) != 1 {
				fail(c, 401, "UNAUTHORIZED", "Provide a valid X-API-Key header")
				return
			}
			// Opt-in local fault injection demonstrates both required 500 responses.
			if testErrors && c.GetHeader("X-Test-Error") == "500" {
				panic("intentional local test error")
			}
			c.Next()
		})
		api.GET("/products/:productId", store.getProduct)
		api.POST("/products/:productId/details", store.updateProduct)
	}
	return r
}

func main() {
	log.SetFlags(0) // Keep request log lines as plain JSON.
	key := os.Getenv("PRODUCT_API_KEY")
	if key == "" {
		key = "hw5-local-demo"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: newRouter(key, strings.EqualFold(os.Getenv("ENABLE_TEST_ERRORS"), "true")), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Fatal(server.ListenAndServe())
}
