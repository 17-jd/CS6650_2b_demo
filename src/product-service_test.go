package main

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const goodProduct = `{"product_id":12345,"sku":"ABC-123-XYZ","manufacturer":"Acme Corporation","category_id":456,"weight":1250,"some_other_id":789}`

func TestContract(t *testing.T) {
	router := newRouter("test-key", true)
	cases := []struct {
		name, method, path, body, fault string
		status                          int
	}{
		{"get existing", "GET", "/products/12345", "", "", 200},
		{"get missing", "GET", "/products/999", "", "", 404},
		{"get bad id", "GET", "/products/0", "", "", 404},
		{"get overflow id", "GET", "/products/2147483648", "", "", 404},
		{"post existing", "POST", "/products/12345/details", goodProduct, "", 204},
		{"post missing", "POST", "/products/999/details", strings.Replace(goodProduct, "12345", "999", 1), "", 404},
		{"post invalid", "POST", "/products/12345/details", `{}`, "", 400},
		{"post null", "POST", "/products/12345/details", `null`, "", 400},
		{"missing weight", "POST", "/products/12345/details", strings.Replace(goodProduct, `"weight":1250,`, "", 1), "", 400},
		{"negative weight", "POST", "/products/12345/details", strings.Replace(goodProduct, "1250", "-1", 1), "", 400},
		{"zero weight", "POST", "/products/12345/details", strings.Replace(goodProduct, "1250", "0", 1), "", 204},
		{"id mismatch", "POST", "/products/1/details", goodProduct, "", 400},
		{"oversize sku", "POST", "/products/12345/details", strings.Replace(goodProduct, "ABC-123-XYZ", strings.Repeat("a", 101), 1), "", 400},
		{"two objects", "POST", "/products/12345/details", goodProduct + goodProduct, "", 400},
		{"get internal error", "GET", "/products/12345", "", "500", 500},
		{"post internal error", "POST", "/products/12345/details", goodProduct, "500", 500},
		{"v1 path", "GET", "/v1/products/12345", "", "", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("X-API-Key", "test-key")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-Error", tc.fault)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("want %d, got %d: %s", tc.status, w.Code, w.Body)
			}
			if w.Code == 204 && w.Body.Len() != 0 {
				t.Fatal("204 must have no body")
			}
		})
	}
	// Check that a successful write is actually returned by the next GET.
	get := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/products/12345", nil)
	req.Header.Set("X-API-Key", "test-key")
	router.ServeHTTP(get, req)
	if !strings.Contains(get.Body.String(), `"weight":0`) {
		t.Fatal("updated weight was not saved")
	}
}

func TestFaultInjectionOffAndAuth(t *testing.T) {
	router := newRouter("test-key", false)
	for _, key := range []string{"", "wrong", "test-key"} {
		req := httptest.NewRequest("GET", "/products/12345", nil)
		req.Header.Set("X-API-Key", key)
		req.Header.Set("X-Test-Error", "500")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		want := 401
		if key == "test-key" {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("want %d, got %d", want, w.Code)
		}
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	router := newRouter("test-key", false)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			method, path, body := "GET", "/products/12345", ""
			if i%2 == 0 {
				method, path, body = "POST", "/products/12345/details", goodProduct
			}
			req := httptest.NewRequest(method, path, strings.NewReader(body))
			req.Header.Set("X-API-Key", "test-key")
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != 200 && w.Code != 204 {
				t.Error(fmt.Sprintf("unexpected status %d", w.Code))
			}
		}(i)
	}
	wg.Wait()
}
