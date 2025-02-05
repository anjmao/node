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

package iptables

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockerSetupIsSuccessful(t *testing.T) {
	mockedExec := &mockedCmdExec{
		mocks: map[string]cmdExecResult{
			"--version": {
				output: []string{"iptables v1.6.0"},
			},
			"-S OUTPUT": {
				output: []string{
					"-P OUTPUT ACCEPT",
				},
			},
		},
	}
	iptablesExec = mockedExec.IptablesExec

	blocker := New("1.1.1.1")
	assert.NoError(t, blocker.Setup())
	assert.True(t, mockedExec.VerifyCalledWithArgs(addChain, killswitchChain))
	assert.True(t, mockedExec.VerifyCalledWithArgs(appendRule, killswitchChain, module, conntrack, ctState, ctStateNew, jumpTo, reject))
}

func TestBlockerSetupIsSucessfulIfPreviousCleanupFailed(t *testing.T) {
	mockedExec := &mockedCmdExec{
		mocks: map[string]cmdExecResult{
			"--version": {
				output: []string{"iptables v1.6.0"},
			},
			"-S OUTPUT": {
				output: []string{
					"-P OUTPUT ACCEPT",
					// leftover - kill switch is still enabled
					"-A OUTPUT -s 5.5.5.5 -j CONSUMER_KILL_SWITCH",
				},
			},
			// kill switch chain still exists
			"-S CONSUMER_KILL_SWITCH": {
				output: []string{
					// with some allowed ips
					"-A CONSUMER_KILL_SWITCH -d 2.2.2.2 -j ACCEPT",
					"-A CONSUMER_KILL_SWITCH -j REJECT",
				},
			},
		},
	}
	iptablesExec = mockedExec.IptablesExec

	blocker := New("1.1.1.1")
	assert.NoError(t, blocker.Setup())
	assert.True(t, mockedExec.VerifyCalledWithArgs(removeRule, outputChain, sourceIP, "5.5.5.5", jumpTo, killswitchChain))
	assert.True(t, mockedExec.VerifyCalledWithArgs(removeChainRules, killswitchChain))
	assert.True(t, mockedExec.VerifyCalledWithArgs(removeChain, killswitchChain))
	assert.True(t, mockedExec.VerifyCalledWithArgs(addChain, killswitchChain))
	assert.True(t, mockedExec.VerifyCalledWithArgs(appendRule, killswitchChain, module, conntrack, ctState, ctStateNew, jumpTo, reject))

}

func TestBlockerResetIsSuccessful(t *testing.T) {
	mockedExec := &mockedCmdExec{
		mocks: map[string]cmdExecResult{
			"-S OUTPUT": {
				output: []string{
					"-P OUTPUT ACCEPT",
					// kill switch is enabled
					"-A OUTPUT -s 1.1.1.1 -j CONSUMER_KILL_SWITCH",
				},
			},
			"-S CONSUMER_KILL_SWITCH": {
				output: []string{
					//first allowed address
					"-A CONSUMER_KILL_SWITCH -d 2.2.2.2 -j ACCEPT",
					//second allowed address
					"-A CONSUMER_KILL_SWITCH -d 3.3.3.3 -j ACCEPT",
					//drop everything else
					"-A CONSUMER_KILL_SWITCH -j REJECT",
				},
			},
		},
	}
	iptablesExec = mockedExec.IptablesExec

	blocker := New("1.1.1.1")
	blocker.Reset()

	assert.True(t, mockedExec.VerifyCalledWithArgs(removeRule, outputChain, sourceIP, "1.1.1.1", jumpTo, killswitchChain))
	assert.True(t, mockedExec.VerifyCalledWithArgs(removeChainRules, killswitchChain))
	assert.True(t, mockedExec.VerifyCalledWithArgs(removeChain, killswitchChain))
}

func TestBlockerBlocksAllOutgoingTraffic(t *testing.T) {
	mockedExec := &mockedCmdExec{
		mocks: map[string]cmdExecResult{},
	}
	iptablesExec = mockedExec.IptablesExec

	blocker := New("1.1.1.1")

	removeRuleFunc, err := blocker.BlockOutgoingTraffic()
	assert.NoError(t, err)
	assert.True(t, mockedExec.VerifyCalledWithArgs(appendRule, outputChain, sourceIP, "1.1.1.1", jumpTo, killswitchChain))

	removeRuleFunc()
	assert.True(t, mockedExec.VerifyCalledWithArgs(removeRule, outputChain, sourceIP, "1.1.1.1", jumpTo, killswitchChain))
}

func TestBlockerAddsAllowedIP(t *testing.T) {
	mockedExec := &mockedCmdExec{
		mocks: map[string]cmdExecResult{},
	}
	iptablesExec = mockedExec.IptablesExec

	blocker := New("1.1.1.1")

	removeRuleFunc, err := blocker.AllowIPAccess("2.2.2.2")
	assert.NoError(t, err)
	assert.True(t, mockedExec.VerifyCalledWithArgs(insertRule, killswitchChain, "1", destinationIP, "2.2.2.2", jumpTo, accept))

	removeRuleFunc()
	assert.True(t, mockedExec.VerifyCalledWithArgs(removeRule, killswitchChain, destinationIP, "2.2.2.2", jumpTo, accept))

}

type cmdExecResult struct {
	called bool
	output []string
	err    error
}

type mockedCmdExec struct {
	mocks map[string]cmdExecResult
}

func (mce *mockedCmdExec) IptablesExec(args ...string) ([]string, error) {
	key := argsToKey(args...)
	res := mce.mocks[key]
	res.called = true
	mce.mocks[key] = res
	return res.output, res.err
}

func (mce *mockedCmdExec) VerifyCalledWithArgs(args ...string) bool {
	key := argsToKey(args...)
	return mce.mocks[key].called
}

func argsToKey(args ...string) string {
	return strings.Join(args, " ")
}

func TestIPResolve(t *testing.T) {
	ips, err := net.LookupHost("216.58.209.3")
	assert.NoError(t, err)
	fmt.Println(ips)
}
