// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	currentRouter *RouterState
)

var (
	requestsPerMinute = 5
	rateLimitWindow   = time.Minute

	// map[ip] = []timestamps
	visitors = make(map[string][]time.Time)
	mu       sync.Mutex
)

func BuildRouter(config *Config) {
	rs := &RouterState{
		m: make(map[string]http.Handler),
		s: make(map[string]*Server),
	}
	for _, server := range config.Servers {
		rs.m[strings.ToLower(server.Name)] = createSingleHttpServer(server)
		rs.s[strings.ToLower(server.Name)] = server
	}

	rs.globalConfig = config
	currentRouter = rs
}

func GetRouter() *RouterState {
	return currentRouter
}

func createSingleHttpServer(s *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.ServeHTTP)

	var middlewares []func(http.Handler) http.Handler

	for _, v := range s.Middlewares {
		m := MiddlewaresList[MiddleWareName(v.Name)]

		middlewares = append(middlewares, m.Fn(v.Config))
	}

	return ChainMiddleware(mux, middlewares...)
}

// ping returns a "pong" message consider registering this Handler for the health checking logic
func Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

func HealthChecker(server *Server) (bool, error) {
	serverURL, err := url.Parse(server.URL)
	var responseBody []byte

	if server.URL == "" || err != nil {
		serverURL, err = url.Parse(fmt.Sprintf("%s://%s:%d", server.Protocol, server.Host, server.Port))
		if err != nil {
			return false, err
		}
	}

	req, err := http.NewRequest("GET", serverURL.String(), &io.LimitedReader{})

	if err != nil {
		return false, err
	}

	client := &http.Client{}

	res, err := client.Do(req)

	if err != nil {
		return false, err
	}

	_, err = res.Body.Read(responseBody)

	if err != nil {
		return false, err
	}

	if res.StatusCode >= 400 {
		return false, fmt.Errorf("server respond with error code: %s body: %s", fmt.Sprint(res.StatusCode), string(responseBody))
	}

	return true, nil
}

type RateLimitMiddlewareConfig struct {
	ReqPerMinute int           `json:"request_per_minute,omitempty" yaml:"request_per_minute,omitempty"`
	LimitWindow  time.Duration `json:"limit_window,omitempty" yaml:"limit_window,omitempty"`
}

func RateLimiterMiddleware(config any) func(next http.Handler) http.Handler {

	conf, ok := config.(*RateLimitMiddlewareConfig)
	if !ok {
		log.Printf("WARNING: RateLimiterMiddleware received config of unexpected type (%T). Defaulting to passthrough handler.", config)
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}
	}

	if conf.ReqPerMinute > 0 {
		requestsPerMinute = conf.ReqPerMinute
	}
	if conf.LimitWindow > 0 {
		rateLimitWindow = conf.LimitWindow
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr // or r.Header["X-Forwarded-For"][0] when the request comes from proxy

			mu.Lock()
			defer mu.Unlock()

			now := time.Now()
			requestTimes := visitors[ip]

			var filtered []time.Time
			for _, t := range requestTimes {
				if now.Sub(t) < rateLimitWindow {
					filtered = append(filtered, t)
				}
			}

			if len(filtered) >= requestsPerMinute {
				http.Error(w, "Max request exceed", http.StatusTooManyRequests)
				return
			}

			filtered = append(filtered, now)
			visitors[ip] = filtered

			next.ServeHTTP(w, r)
		})
	}
}

func CleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		mu.Lock()
		for ip, timestamps := range visitors {
			var filtered []time.Time
			for _, t := range timestamps {
				if time.Since(t) < rateLimitWindow {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) == 0 {
				delete(visitors, ip)
			} else {
				visitors[ip] = filtered
			}
		}
		mu.Unlock()
	}
}

func ChainMiddleware(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

func routeHandler(w http.ResponseWriter, r *http.Request) {
	rs := GetRouter()
	if rs == nil {
		http.Error(w, "router not ready", http.StatusServiceUnavailable)
		return
	}
	b, ok := rs.m[strings.ToLower(r.Host)]
	if !ok {
		http.NotFound(w, r)
		return
	}
	b.ServeHTTP(w, r)
}

func httpEntry(w http.ResponseWriter, r *http.Request) {
	rs := GetRouter()
	if rs == nil {
		http.Error(w, "router not ready", http.StatusServiceUnavailable)
		return
	}
	if b, ok := rs.s[strings.ToLower(r.Host)]; ok && b.forceTLS {
		url := *r.URL
		url.Scheme = "https"
		url.Host = r.Host
		http.Redirect(w, r, url.String(), http.StatusMovedPermanently)
		return
	}
	routeHandler(w, r)
}

func ServeHTTP(addr string) *http.Server {
	hs := &http.Server{Addr: addr, Handler: http.HandlerFunc(httpEntry)}
	go func() {
		log.Printf("HTTP listening on %s", addr)
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()
	return hs
}

func ServeHTTPS(addr string, cm *CertManager) *http.Server {
	ts := &http.Server{Addr: addr, Handler: http.HandlerFunc(routeHandler)}
	ts.TLSConfig = &tls.Config{GetCertificate: cm.GetCertificate, MinVersion: tls.VersionTLS12}
	go func() {
		log.Printf("HTTPS listening on %s", addr)
		ln, err := tls.Listen("tcp", addr, ts.TLSConfig)
		if err != nil {
			log.Fatalf("https listen: %v", err)
		}
		if err := ts.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("https server: %v", err)
		}
	}()
	return ts
}
