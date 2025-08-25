// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIntegration_ConfigParsing(t *testing.T) {
	jsonContent := []byte(`{"server":[{"name":"s1","protocol":"http","host":"localhost","port":8080}]}`)
	cfg, err := ParseConfig(jsonContent, "json")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cfg.Servers))

	badContent := []byte(`{invalid}`)
	_, err2 := ParseConfig(badContent, "json")
	assert.Error(t, err2)

	_, err3 := ParseConfig(jsonContent, "xml")
	assert.Error(t, err3)
}

func TestIntegration_HealthChecker_Error(t *testing.T) {
	server := &Server{Name: "bad", URL: "http://invalid:9999"}
	ok, err := HealthChecker(server)
	assert.False(t, ok)
	assert.Error(t, err)
}

func TestIntegration_LoadBalancerWithHealthCheck(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("healthy"))
	}))
	defer healthy.Close()

	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	defer unhealthy.Close()

	server := &Server{Name: "main"}
	serverHealthy := &Server{Name: "healthy", URL: healthy.URL}
	serverUnhealthy := &Server{Name: "unhealthy", URL: unhealthy.URL}

	_ = serverHealthy.UpgradeProxy()
	_ = serverUnhealthy.UpgradeProxy()

	server.AddNewBalancingServer(serverHealthy)
	server.AddNewBalancingServer(serverUnhealthy)

	// Health check all
	status, err := server.CheckHealthAll()
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.True(t, len(status.Pass)+len(status.Fail) == 2)

	// Use only healthy server for load balancer
	server.BalancingServers = []*Server{serverHealthy}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	resp := w.Result()
	assert.Equal(t, 200, resp.StatusCode)
}
