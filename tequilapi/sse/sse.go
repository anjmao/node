/*
 * Copyright (C) 2019 The "MysteriumNetwork/node" Authors.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	log "github.com/cihub/seelog"
	"github.com/julienschmidt/httprouter"
	nodeEvent "github.com/mysteriumnetwork/node/core/node/event"
	stateEvent "github.com/mysteriumnetwork/node/core/state/event"
	"github.com/pkg/errors"
)

const logPrefix = "[sse-handler]"

// EventType represents all the event types we're subscribing to
type EventType string

// Event represents an event we're gonna send
type Event struct {
	Payload interface{} `json:"payload"`
	Type    EventType   `json:"type"`
}

const (
	// NATEvent represents the nat event type
	NATEvent EventType = "nat"
	// ServiceStatusEvent represents the service status event type
	ServiceStatusEvent EventType = "service-status"
	// StateChangeEvent represents the state change
	StateChangeEvent EventType = "state-change"
)

// Handler represents an sse handler
type Handler struct {
	clients       map[chan string]struct{}
	newClients    chan chan string
	deadClients   chan chan string
	messages      chan string
	stopOnce      sync.Once
	stopChan      chan struct{}
	stateProvider stateProvider
}

type stateProvider interface {
	GetState() stateEvent.State
}

// NewHandler returns a new instance of handler
func NewHandler(stateProvider stateProvider) *Handler {
	return &Handler{
		clients:       make(map[chan string]struct{}),
		newClients:    make(chan (chan string)),
		deadClients:   make(chan (chan string)),
		messages:      make(chan string, 20),
		stopChan:      make(chan struct{}),
		stateProvider: stateProvider,
	}
}

// Sub subscribes a user to sse
func (h *Handler) Sub(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
	f, ok := resp.(http.Flusher)
	if !ok {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Header().Set("Content-type", "application/json; charset=utf-8")
		writeErr := json.NewEncoder(resp).Encode(errors.New("not a flusher - cannot continue"))
		if writeErr != nil {
			http.Error(resp, "Http response write error", http.StatusInternalServerError)
		}
		return
	}

	resp.Header().Set("Content-Type", "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")

	messageChan := make(chan string, 1)
	err := h.sendInitialState(messageChan)
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Header().Set("Content-type", "application/json; charset=utf-8")
		writeErr := json.NewEncoder(resp).Encode(err)
		if writeErr != nil {
			http.Error(resp, "Http response write error", http.StatusInternalServerError)
		}
	}

	h.newClients <- messageChan

	go func() {
		<-req.Context().Done()
		h.deadClients <- messageChan
	}()

	for {
		select {
		case msg, open := <-messageChan:
			if !open {
				return
			}

			_, err := fmt.Fprintf(resp, "data: %s\n\n", msg)
			if err != nil {
				log.Error(logPrefix, err)
				return
			}

			f.Flush()
		case <-h.stopChan:
			return
		}
	}
}

func (h *Handler) sendInitialState(messageChan chan string) error {
	res, err := json.Marshal(Event{
		Type:    StateChangeEvent,
		Payload: h.stateProvider.GetState(),
	})
	if err != nil {
		return err
	}

	messageChan <- string(res)
	return nil
}

func (h *Handler) serve() {
	defer func() {
		for k := range h.clients {
			close(k)
		}
	}()

	for {
		select {
		case <-h.stopChan:
			return
		case s := <-h.newClients:
			h.clients[s] = struct{}{}
		case s := <-h.deadClients:
			delete(h.clients, s)
			close(s)
		case msg := <-h.messages:
			for s := range h.clients {
				s <- msg
			}
		}
	}
}

func (h *Handler) stop() {
	h.stopOnce.Do(func() { close(h.stopChan) })
}

func (h *Handler) send(e Event) {
	marshaled, err := json.Marshal(e)
	if err != nil {
		log.Error(logPrefix, "could not marshal sse message", err)
		return
	}
	h.messages <- string(marshaled)
}

// ConsumeNodeEvent consumes the node state event
func (h *Handler) ConsumeNodeEvent(e nodeEvent.Payload) {
	if e.Status == nodeEvent.StatusStarted {
		go h.serve()
		return
	}
	if e.Status == nodeEvent.StatusStopped {
		h.stop()
		return
	}
}

// ConsumeStateEvent consumes the state change event
func (h *Handler) ConsumeStateEvent(event stateEvent.State) {
	h.send(Event{
		Type:    StateChangeEvent,
		Payload: event,
	})
}
