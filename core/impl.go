// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// UpgradeProxy ensures server.Proxy is initialized for this server.
func (server *Server) UpgradeProxy() error {
	if server == nil {
		return errors.New("nil receiver: server")
	}
	if server.proxy != nil {
		return nil
	}
	serverURL, err := buildServerURL(server)
	if err != nil {
		return err
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return err
	}
	server.proxy = NewProxy(u)
	return nil
}

func (server *Server) AddNewBalancingServer(bs *Server) {
	if bs == nil {
		return
	}
	server.BalancingServers = append(server.BalancingServers, bs)
}

func (server *Server) DelBalancingServer(name string) {
	if name == "" {
		return
	}
	filtered := server.BalancingServers[:0]
	for _, s := range server.BalancingServers {
		if s != nil && s.Name != name {
			filtered = append(filtered, s)
		}
	}
	server.BalancingServers = filtered
}

func (server *Server) GetServer(name string) *Server {
	if name == "" {
		return nil
	}
	for _, s := range server.BalancingServers {
		if s != nil && s.Name == name {
			return s
		}
	}
	return nil
}

// CheckHealthAll performs health checks without holding the lock during network I/O.
func (server *Server) CheckHealthAll() (*HealthCheckStatus, error) {
	server.mu.Lock()
	servers := slices.Clone(server.BalancingServers)
	server.mu.Unlock()

	var hc HealthCheckStatus
	start := time.Now()

	for _, target := range servers {
		if target == nil {
			continue
		}
		checkStart := time.Now()
		success, err := HealthChecker(target)
		u, _ := buildServerURL(target)

		// Update target's fields under its own lock if it has one; otherwise assume Server.mu protects it.
		server.mu.Lock()
		target.IsHealthy = err == nil && success
		target.LastHealthCheck = func(t time.Time) *time.Time { return &t }(checkStart)
		server.mu.Unlock()

		entry := ServerStatus{Name: target.Name, Url: u, Healthy: target.IsHealthy}
		if target.IsHealthy {
			hc.Pass = append(hc.Pass, entry)
		} else {
			hc.Fail = append(hc.Fail, entry)
		}
	}

	hc.Duration = time.Since(start)
	return &hc, nil
}

func (server *Server) CheckHealthAny(name string) (*ServerStatus, error) {
	if name == "" {
		return nil, fmt.Errorf("empty server name")
	}
	server.mu.Lock()
	target := server.GetServer(name)
	server.mu.Unlock()
	if target == nil {
		return nil, fmt.Errorf("no server found for name %q", name)
	}
	u, _ := buildServerURL(target)
	success, err := HealthChecker(target)
	healthy := err == nil && success
	return &ServerStatus{Name: target.Name, Url: u, Healthy: healthy}, err
}

func (server *Server) CheckHealthSelf() (*ServerStatus, error) {
	u, _ := buildServerURL(server)
	success, err := HealthChecker(server)
	healthy := err == nil && success
	return &ServerStatus{Name: server.Name, Url: u, Healthy: healthy}, err
}

// RollBack replaces the current balancing set with the provided list atomically.
func (server *Server) RollBack(servers []*Server) {
	server.mu.Lock()
	server.BalancingServers = slices.Clone(servers)
	server.mu.Unlock()
}

// RollBackAny replaces a named server with a new one, or appends when name is empty.
func (server *Server) RollBackAny(name string, newServer *Server) error {
	if newServer == nil {
		return fmt.Errorf("newServer must not be nil")
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if name == "" {
		server.AddNewBalancingServer(newServer)
		return nil
	}
	for i, s := range server.BalancingServers {
		if s != nil && s.Name == name {
			server.BalancingServers[i] = newServer
			return nil
		}
	}
	return fmt.Errorf("server named %q not found", name)
}

// GetNextServer returns the next healthy server using round-robin.
// If all are unhealthy or list is empty, an error is returned.
func (server *Server) GetNextServer() (*Server, error) {
	server.mu.Lock()
	defer server.mu.Unlock()

	n := len(server.BalancingServers)
	if n == 0 {
		return nil, fmt.Errorf("no backend servers configured")
	}
	// Iterate at most n times to find a healthy server.
	for range n {
		server.idx = (server.idx + 1) % n
		cand := server.BalancingServers[server.idx]
		if cand != nil && cand.IsHealthy { // prefer healthy
			return cand, nil
		}
	}
	// Fallback: return next even if unhealthy to avoid total outage
	cand := server.BalancingServers[server.idx]
	if cand != nil {
		return cand, nil
	}
	return nil, fmt.Errorf("no usable backend server found")
}

// ServeHTTP implements a minimal LB+proxy. It avoids mutating the receiver on retry
// and constructs the target URL using ResolveReference to handle paths and queries correctly.
func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		backend *Server
		err     error
	)

	server.logf(LOG_INFO, "[Proxy]: New incoming request <- %s Method: %s", r.URL.Path, r.Method)
	server.logf(LOG_INFO, "[Load Balancer]: Selecting next server")

	if len(server.BalancingServers) > 0 {
		backend, err = server.GetNextServer()
		if err != nil {
			server.logf(LOG_ERROR, "[Load Balancer]: %v", err)
			http.Error(w, "No backend available", http.StatusServiceUnavailable)
			return
		}
		if upErr := backend.UpgradeProxy(); upErr != nil {
			server.logf(LOG_ERROR, "failed to init backend proxy: %v", upErr)
		}
	} else {
		// Single-node mode: proxy to self
		backend = server
		if upErr := server.UpgradeProxy(); upErr != nil {
			server.logf(LOG_ERROR, "failed to init proxy: %v", upErr)
		}
	}

	baseURL, err := parseServerURL(backend)
	if err != nil {
		server.logf(LOG_ERROR, "[Proxy]: invalid backend URL for %s: %v", backend.Name, err)
		http.Error(w, "Invalid backend url", http.StatusInternalServerError)
		return
	}

	// Build destination URL preserving path and query
	joinedPath := singleSlashJoin(baseURL.Path, r.URL.Path)
	target := *baseURL
	target.Path = joinedPath
	target.RawQuery = r.URL.RawQuery

	// Clone request with context and body; copy headers
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), r.Body)
	if err != nil {
		server.logf(LOG_ERROR, "[Fatal]: cannot create outbound request: %v", err)
		http.Error(w, "Failed to create backend request", http.StatusInternalServerError)
		return
	}
	req.Header = r.Header.Clone()
	appendForwardHeaders(req.Header, r, baseURL.Scheme)

	server.logf(LOG_INFO, "[Proxy]: Forwarding %s -> %s (backend ID: %s)", r.URL.String(), target.String(), backend.ID)

	// Delegate to the preconfigured reverse proxy for the backend.
	backend.proxy.ServeHTTP(w, req)
}

// Router

func (rs *RouterState) AddServer(server *Server) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.m[strings.ToLower(server.Name)] = createSingleHttpServer(server)
	rs.s[strings.ToLower(server.Name)] = server
}

// --- helpers ---

func (s *Server) logf(level int, format string, args ...any) {
	currentLogger := GetLogger()
	if s == nil || currentLogger == nil {
		return
	}
	// Non-blocking send; drop if channel is full
	msg := fmt.Sprintf(format, args...)
	select {
	case currentLogger <- Logs{Message: msg, LogType: level}:
	default:
	}
}

func parseServerURL(s *Server) (*url.URL, error) {
	if s.URL != "" {
		u, err := url.Parse(s.URL)
		if err == nil {
			return u, nil
		}
	}
	built, err := buildServerURL(s)
	if err != nil {
		return nil, err
	}
	return url.Parse(built)
}

func singleSlashJoin(a, b string) string {
	slashA := strings.HasSuffix(a, "/")
	prefixB := strings.HasPrefix(b, "/")
	switch {
	case slashA && prefixB:
		return a + strings.TrimPrefix(b, "/")
	case !slashA && !prefixB:
		return a + "/" + b
	default:
		return a + b
	}
}

func appendForwardHeaders(h http.Header, r *http.Request, scheme string) {
	ip := clientIP(r.RemoteAddr)
	if prior := h.Get("X-Forwarded-For"); prior != "" {
		h.Set("X-Forwarded-For", prior+", "+ip)
	} else {
		h.Set("X-Forwarded-For", ip)
	}
	h.Set("X-Forwarded-Proto", scheme)
	if r.Host != "" {
		h.Set("X-Forwarded-Host", r.Host)
	}
}

func clientIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
