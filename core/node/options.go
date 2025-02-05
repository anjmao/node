/*
 * Copyright (C) 2018 The "MysteriumNetwork/node" Authors.
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

import "github.com/mysteriumnetwork/node/logconfig"

// Openvpn interface is abstraction over real openvpn options to unblock mobile development
// will disappear as soon as go-openvpn will unify common factory for openvpn creation
type Openvpn interface {
	Check() error
	BinaryPath() string
}

// Options describes options which are required to start Node
type Options struct {
	Directories OptionsDirectory

	TequilapiAddress string
	TequilapiPort    int
	BindAddress      string
	UI               OptionsUI

	DisableMetrics bool
	MetricsAddress string

	Keystore OptionsKeystore

	logconfig.LogOptions
	OptionsNetwork
	Discovery  OptionsDiscovery
	Quality    OptionsQuality
	Location   OptionsLocation
	Transactor OptionsTransactor

	Openvpn  Openvpn
	Firewall OptionsFirewall
}

// OptionsKeystore stores the keystore configuration
type OptionsKeystore struct {
	UseLightweight bool
}
