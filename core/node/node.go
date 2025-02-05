/*
 * Copyright (C) 2017 The "MysteriumNetwork/node" Authors.
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

package node

import (
	"github.com/mysteriumnetwork/node/core/connection"
	"github.com/mysteriumnetwork/node/core/node/event"
	"github.com/mysteriumnetwork/node/tequilapi"
)

// NatPinger allows to send nat pings as well as stop it
type NatPinger interface {
	Start()
	Stop()
}

// Publisher is responsible for publishing given events
type Publisher interface {
	Publish(topic string, data interface{})
}

// UIServer represents the web server for our web
type UIServer interface {
	Serve() error
	Stop()
}

// NewNode function creates new Mysterium node by given options
func NewNode(
	connectionManager connection.Manager,
	tequilapiServer tequilapi.APIServer,
	publisher Publisher,
	natPinger NatPinger,
	uiServer UIServer,
) *Node {
	return &Node{
		connectionManager: connectionManager,
		httpAPIServer:     tequilapiServer,
		publisher:         publisher,
		natPinger:         natPinger,
		uiServer:          uiServer,
	}
}

// Node represent entrypoint for Mysterium node with top level components
type Node struct {
	connectionManager connection.Manager
	httpAPIServer     tequilapi.APIServer
	publisher         Publisher
	natPinger         NatPinger
	uiServer          UIServer
}

// Start starts Mysterium node (Tequilapi service, fetches location)
func (node *Node) Start() error {
	node.httpAPIServer.StartServing()
	address, err := node.httpAPIServer.Address()
	if err != nil {
		return err
	}

	go func() {
		err := node.uiServer.Serve()
		if err != nil {
			log.Error("UI server error", err)
		}
	}()

	node.publisher.Publish(event.Topic, event.Payload{Status: event.StatusStarted})

	log.Infof("API started on: %v", address)
	go node.natPinger.Start()

	return nil
}

// Wait blocks until Mysterium node is stopped
func (node *Node) Wait() error {
	defer node.publisher.Publish(event.Topic, event.Payload{Status: event.StatusStopped})
	return node.httpAPIServer.Wait()
}

// Kill stops Mysterium node
func (node *Node) Kill() error {
	err := node.connectionManager.Disconnect()
	if err != nil {
		switch err {
		case connection.ErrNoConnection:
			log.Info("no active connection - proceeding")
		default:
			return err
		}
	} else {
		log.Info("connection closed")
	}

	node.httpAPIServer.Stop()
	log.Info("API stopped")

	node.uiServer.Stop()
	log.Info("web server stopped")

	node.natPinger.Stop()
	log.Info("NAT pinger stopped")

	return nil
}
