// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	certmagic "github.com/caddyserver/certmagic"
)

type LogType = int

const (
	LOG_INFO LogType = iota
	LOG_DEBUG
	LOG_ERROR
)

type Server struct {
	ID               string       // THe server ID based on its registration order
	Name             string       `json:"name,omitempty" yaml:"name,omitempty"`             // The server name
	Protocol         string       `json:"protocol,omitempty" yaml:"protocol,omitempty"`     // The protocol for the server this field can be `http` or `https`
	Host             string       `json:"host,omitempty" yaml:"host,omitempty"`             // The server host
	Port             int          `json:"port,omitempty" yaml:"port,omitempty"`             // The port on which the server is running
	URL              string       `json:"url,omitempty" yaml:"url,omitempty"`               // If this field is provided the URL will be used for request forwarding
	IsHealthy        bool         `json:"is_healthy,omitempty" yaml:"is_healthy,omitempty"` // Specifying the server health check state
	BalancingServers []*Server    `json:"balance,omitempty" yaml:"balance,omitempty"`       // If specified these servers will be used for load balancing request
	Middlewares      []Middleware `json:"middlewares,omitempty" yaml:"middlewares,omitempty"`
	LastHealthCheck  *time.Time
	proxy            *httputil.ReverseProxy
	mu               sync.Mutex
	idx              int
	forceTLS         bool
}

type Middleware struct {
	Name   string `json:"name" yaml:"name"`
	Config any    `json:"config,omitempty" yaml:"config,omitempty"`
}

type BalancerStrategy interface {
	GetNextServer() (*Server, error)
}

type Logs struct {
	Target  string
	LogType LogType
	Message string
}

type Config struct {
	Servers             []*Server `json:"server" yaml:"server"` // The servers instances
	HealthCheckInterval int       `json:"healthcheck_interval,omitempty" yaml:"healthcheck_interval,omitempty"`
	LogOutput           string    `json:"log_output,omitempty" yaml:"log_output,omitempty"`
}

// The result of a health checking process for a server
type ServerStatus struct {
	Name    string `json:"name" yaml:"name"`       // The server name
	Url     string `json:"url" yaml:"url"`         // HealthCheck url
	Healthy bool   `json:"healthy" yaml:"healthy"` // healthCheck status
}

type HealthCheckStatus struct {
	Pass      []ServerStatus `json:"pass" yaml:"pass"` // Array of successful HealthCheck result
	Fail      []ServerStatus `json:"fail" yaml:"fail"` // Array of failure HealthCheck Result
	CheckTime time.Time
	Duration  time.Duration
}

type MogolyMiddleware func(config any) func(next http.Handler) http.Handler

type MiddleWareName string

type MiddlewareSets map[MiddleWareName]struct {
	Fn   MogolyMiddleware
	Conf any
}

type DNSServer struct {
	isLocal   func(string) bool
	forwardTo string // optional upstream (ip:port)
}

type CertManager struct {
	cm        *certmagic.Config
	selfStore map[string]*tls.Certificate // cache for local/self-signed
	mu        sync.RWMutex
}

type RouterState struct {
	mu           sync.RWMutex
	m            map[string]http.Handler // host -> backend
	s            map[string]*Server
	globalConfig *Config
}

type ConfigRules map[string]ConfigRulesTarget

type ConfigRulesTarget struct {
	Conf    *Config
	Targets []*Server
}

type Logger chan any
