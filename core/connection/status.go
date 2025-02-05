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

package connection

import (
	"github.com/mysteriumnetwork/node/market"
	"github.com/mysteriumnetwork/node/session"
)

// State represents list of possible connection states
type State string

const (
	// NotConnected means no connection exists
	NotConnected = State("NotConnected")
	// Connecting means that connection is startCalled but not yet fully established
	Connecting = State("Connecting")
	// Connected means that fully established connection exists
	Connected = State("Connected")
	// Disconnecting means that connection close is in progress
	Disconnecting = State("Disconnecting")
	// Reconnecting means that connection is lost but underlying service is trying to reestablish it
	Reconnecting = State("Reconnecting")
	// Unknown means that we could not map the underlying transport state to our state
	Unknown = State("Unknown")
	// Canceled means that connection initialization was started, but failed never reaching Connected state
	Canceled = State("Canceled")
)

// Status holds connection state, session id and proposal of the connection
type Status struct {
	State     State
	SessionID session.ID
	Proposal  market.ServiceProposal
}

func statusConnecting() Status {
	return Status{State: Connecting}
}

func statusConnected(sessionID session.ID, proposal market.ServiceProposal) Status {
	return Status{Connected, sessionID, proposal}
}

func statusNotConnected() Status {
	return Status{State: NotConnected}
}

func statusReconnecting() Status {
	return Status{State: Reconnecting}
}

func statusDisconnecting() Status {
	return Status{State: Disconnecting}
}

func statusCanceled() Status {
	return Status{State: Canceled}
}
