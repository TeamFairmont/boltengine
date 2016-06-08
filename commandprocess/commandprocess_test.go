// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package commandprocess

import (
	"testing"
	"time"

	"github.com/TeamFairmont/gabs"
	"github.com/stretchr/testify/assert"
)

func TestNewCommandProcessWithID(t *testing.T) {
	cp := NewCommandProcessWithID("123", CallTypeTask, "cmd", nil, nil, "group", "token")
	assert.Equal(t, "123", cp.ID, "UUID matches")
	assert.Equal(t, CallTypeTask, cp.CallType, "CallType matches")
	assert.Equal(t, "group", cp.HMACGroup, "HMACGroup matches")
	assert.Equal(t, "token", cp.HMACToken, "HMACToken matches")
	assert.Equal(t, "cmd", cp.InitialCommand, "InitialCommand matches")
}

func TestNewCommandProcess(t *testing.T) {
	cp := NewCommandProcess(CallTypeTask, "cmd", nil, nil, "group", "token")
	assert.NotEqual(t, "", cp.ID, "UUID SHOULDNT match")
	assert.Equal(t, CallTypeTask, cp.CallType, "CallType matches")
	assert.Equal(t, "group", cp.HMACGroup, "HMACGroup matches")
	assert.Equal(t, "token", cp.HMACToken, "HMACToken matches")
	assert.Equal(t, "cmd", cp.InitialCommand, "InitialCommand matches")
}

func TestStartTimeout(t *testing.T) {

}

func TestUpdatePeekTime(t *testing.T) {
	now := time.Now()
	time.Sleep(time.Millisecond)
	cp := NewCommandProcess(CallTypeTask, "cmd", nil, nil, "group", "token")
	cp.UpdatePeekTime()

	assert.Condition(t, func() bool {
		return cp.PeekTime.After(now)
	}, "Complete time should be greater than or equal to test start time")
}

func TestSetComplete(t *testing.T) {
	now := time.Now()
	time.Sleep(time.Millisecond)
	cp := NewCommandProcess(CallTypeTask, "cmd", nil, nil, "group", "token")
	cp.SetComplete()

	assert.True(t, cp.Complete, "Complete should be true")
	assert.Condition(t, func() bool {
		return cp.CompleteTime.After(now)
	}, "Complete time should be greater than or equal to test start time")
}

func TestAddTraceEntry(t *testing.T) {
	payload, _ := gabs.ParseJSON([]byte(EmptyPayload))
	cp := NewCommandProcess(CallTypeTask, "cmd", nil, payload, "group", "token")
	cp.AddTraceEntry()
	cp.AddTraceEntry()
	count, _ := cp.Payload.ArrayCount("trace")
	assert.Exactly(t, 2, count, "Should be two trace entries")
}
