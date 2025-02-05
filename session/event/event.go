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

package event

// Topic represents the session change topic
const Topic = "Session change"

// DataTransfered represents the data transfer topic
const DataTransfered = "Session data transfered"

// DataTransferEventPayload represents the data transfer event
type DataTransferEventPayload struct {
	ID       string
	Up, Down int64
}

// Action represents the different actions that might happen on a session
type Action string

const (
	// Created indicates a session has been created
	Created Action = "Created"
	// Removed indicates a session has been removed
	Removed Action = "Removed"
	// Updated indicates a session has been updated
	Updated Action = "Updated"
	// Acknowledged indicates a session has been reported as a success from consumer side
	Acknowledged Action = "Acknowledged"
)

// Payload represents the event payload
type Payload struct {
	Action Action
	ID     string
}
