// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sync

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// Creating a new client for a websocket connection.
func NewClient() *Client {
	return &Client{
		Incoming:        make(chan *Message, 100), // Buffer for incoming messages
		dialer:          websocket.DefaultDialer,
		pendingRequests: make(map[string]chan *Message),
	}
}

func NewServer(onMsg HandlerFunc, originChecker func (r *http.Request) bool) *Server {
	server :=  &Server{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				log.Printf("CheckOrigin: Checking origin %s\n", r.Header.Get("Origin"))
				return originChecker(r)
			},
		},
		msgHandler: onMsg,
	}

	server.hub = newHub(server.handleMessage)

	return server
}

func NewErrorMessage(errMsg, details string) *Message {
	payloadBytes, _ := json.Marshal(ErrorPayload{Details: details})
	return &Message{
		Action: Action{
			Type:    ERROR,
			Payload: payloadBytes,
		},
		Error: errMsg,
	}
}

func NewAction(actionType Action_Type) *Action {
	return &Action{
		Type: actionType,
	}
}

func NewMessage(actionType Action_Type, payload any, metaData any) (*Message, error) {
	var payloadBytes []byte
	var metaDataBytes []byte
	var err error
	var message Message

	if payload != nil {
		if payloadBytes, err = json.Marshal(payload); err != nil {
			return nil, err
		}
		message.Action.Payload = payloadBytes
	}

	if metaData != nil {
		if metaDataBytes, err = json.Marshal(metaData); err == nil {
			message.Meta = metaDataBytes
		}
	}

	message.Action.Type = actionType

	return &message, nil
}
