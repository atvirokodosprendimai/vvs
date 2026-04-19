package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIPRateLimiter_AllowsUpToLimit(t *testing.T) {
	limiter := NewIPRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !limiter.Allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if limiter.Allow("1.2.3.4") {
		t.Fatal("4th request should be denied")
	}
}

func TestIPRateLimiter_DifferentIPsIndependent(t *testing.T) {
	limiter := NewIPRateLimiter(2, time.Minute)

	limiter.Allow("1.1.1.1")
	limiter.Allow("1.1.1.1")
	if limiter.Allow("1.1.1.1") {
		t.Fatal("3rd request from same IP should be denied")
	}
	if !limiter.Allow("2.2.2.2") {
		t.Fatal("first request from different IP should be allowed")
	}
}

func TestIPRateLimiter_WindowResets(t *testing.T) {
	limiter := NewIPRateLimiter(1, 10*time.Millisecond)

	if !limiter.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if limiter.Allow("1.2.3.4") {
		t.Fatal("second request should be denied within window")
	}

	time.Sleep(15 * time.Millisecond) // wait for window to expire

	if !limiter.Allow("1.2.3.4") {
		t.Fatal("request after window reset should be allowed")
	}
}

func TestIPRateLimiter_Middleware_Returns429(t *testing.T) {
	limiter := NewIPRateLimiter(2, time.Minute)
	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	makeReq := func() int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	if makeReq() != http.StatusOK {
		t.Fatal("1st request should be 200")
	}
	if makeReq() != http.StatusOK {
		t.Fatal("2nd request should be 200")
	}
	code := makeReq()
	if code != http.StatusTooManyRequests {
		t.Fatalf("3rd request should be 429; got %d", code)
	}
}
