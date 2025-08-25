// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sync

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *Connection // Shared wrapper for WebSocket connection

	// Incoming messages are pushed here by the readPump.
	// Users can read from this channel to process incoming messages.
	Incoming chan *Message // Public channel for incoming messages

	mu          sync.Mutex
	isConnected bool
	dialer      *websocket.Dialer
	connUrl     string
	headers     http.Header // For authentication or other headers

	// pendingRequests holds channels for requests that are waiting for a response.
	// Keyed by RequestID, so we can correlate responses.
	// This allows us to handle responses to specific requests.
	pendingRequests map[string]chan *Message
	pendingMu       sync.RWMutex
}

type HandlerFunc func(msg *Message, conn *Connection) error

type Server struct {
	upgrader   websocket.Upgrader
	hub        *Hub
	msgHandler HandlerFunc
}

type ErrorPayload struct {
	Code    int    `json:"code,omitempty"`
	Details string `json:"details"`
}

// Represents a message payload sending between the server and the client
type Message struct {
	RequestID string `json:"request_id"`
	Action    Action `json:"action"`
	Meta      []byte `json:"meta"`
	Error     string `json:"error"`
}

type RunTaskPayload struct {
	TaskName string `json:"task_name"`
}

type TaskStatusPayload struct {
	TaskName string `json:"task_name"`
	Status   string `json:"status"`
}
