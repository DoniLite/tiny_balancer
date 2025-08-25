// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServer_GetNextServer(t *testing.T) {
	server := &Server{Name: "main"}
	s1 := &Server{Name: "s1", IsHealthy: true}
	s2 := &Server{Name: "s2", IsHealthy: true}
	server.AddNewBalancingServer(s1)
	server.AddNewBalancingServer(s2)
	server.idx = 0

	next1, err := server.GetNextServer()
	assert.NoError(t, err)
	assert.Equal(t, s2, next1)
	next2, err := server.GetNextServer()
	assert.NoError(t, err)
	assert.Equal(t, s1, next2)
}

func TestServer_ServeHTTP(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer backend.Close()

	balancingServer := &Server{Name: "backend", URL: backend.URL}
	_ = balancingServer.UpgradeProxy()

	server := &Server{Name: "backend", URL: backend.URL}
	_ = server.UpgradeProxy()

	server.AddNewBalancingServer(balancingServer)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	resp := w.Result()
	assert.Equal(t, 200, resp.StatusCode)
}

func TestServer_UpgradeProxy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer backend.Close()
	server := &Server{Name: "test", URL: backend.URL}
	err := server.UpgradeProxy()
	assert.NoError(t, err)
	assert.NotNil(t, server.proxy)
}

func TestPing(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ping", nil)
	Ping(w, r)
	assert.Equal(t, "pong", w.Body.String())
}

func TestBuildServerURL(t *testing.T) {
	server := &Server{Protocol: "http", Host: "localhost", Port: 8080}
	url, err := buildServerURL(server)
	assert.NoError(t, err)
	assert.Contains(t, url, "http://localhost:8080")

	server2 := &Server{URL: "http://127.0.0.1:9000"}
	url2, err2 := buildServerURL(server2)
	assert.NoError(t, err2)
	assert.Equal(t, "http://127.0.0.1:9000", url2)

	server3 := &Server{}
	url3, err3 := buildServerURL(server3)
	assert.NoError(t, err3)
	assert.Empty(t, url3)
}

func TestSerializeHealthCheckStatus(t *testing.T) {
	status := &HealthCheckStatus{
		Pass:     []ServerStatus{{Name: "s1", Url: "u1", Healthy: true}},
		Fail:     []ServerStatus{{Name: "s2", Url: "u2", Healthy: false}},
		Duration: time.Second,
	}
	s, err := SerializeHealthCheckStatus(status)
	assert.NoError(t, err)
	assert.Contains(t, s, "s1")
	assert.Contains(t, s, "s2")

	s2, err2 := SerializeHealthCheckStatus(nil)
	assert.NoError(t, err2)
	assert.Equal(t, "null", s2)
}

func TestServer_RollBackAndRollBackAny(t *testing.T) {
	server := &Server{Name: "main"}
	s1 := &Server{Name: "s1"}
	s2 := &Server{Name: "s2"}
	server.BalancingServers = []*Server{s1}

	server.RollBack([]*Server{s2})
	assert.Len(t, server.BalancingServers, 1)
	assert.Equal(t, "s2", server.BalancingServers[0].Name)

	err := server.RollBackAny("s2", s1)
	assert.NoError(t, err)
	assert.Equal(t, "s1", server.BalancingServers[0].Name)

	err2 := server.RollBackAny("s1", nil)
	assert.Error(t, err2)

	err3 := server.RollBackAny("", nil)
	assert.Error(t, err3)
}

func TestServer_CheckHealthSelfAndAny(t *testing.T) {
	// Backend healthy
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer backend.Close()
	server := &Server{Name: "healthy", URL: backend.URL}
	status, err := server.CheckHealthSelf()
	assert.NoError(t, err)
	assert.True(t, status.Healthy)

	// Backend unhealthy
	badServer := &Server{Name: "bad", URL: "http://invalid:9999"}
	status2, err2 := badServer.CheckHealthSelf()
	assert.Error(t, err2)
	assert.False(t, status2.Healthy)

	main := &Server{Name: "main"}
	main.BalancingServers = []*Server{server, badServer}
	st, err3 := main.CheckHealthAny("healthy")
	assert.NoError(t, err3)
	assert.True(t, st.Healthy)
	st2, err4 := main.CheckHealthAny("bad")
	assert.Error(t, err4)
	assert.False(t, st2.Healthy)
}

func TestServer_AddDelGetBalancingServer(t *testing.T) {
	server := &Server{Name: "main"}
	s1 := &Server{Name: "s1"}
	s2 := &Server{Name: "s2"}
	server.AddNewBalancingServer(s1)
	server.AddNewBalancingServer(s2)
	assert.Len(t, server.BalancingServers, 2)
	server.DelBalancingServer("s1")
	assert.Len(t, server.BalancingServers, 1)
	assert.Equal(t, "s2", server.BalancingServers[0].Name)
	got := server.GetServer("s2")
	assert.Equal(t, s2, got)
}
