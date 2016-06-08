// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package requestmanager

import (
	"testing"
	"time"

	"github.com/TeamFairmont/boltengine/commandprocess"
	"github.com/TeamFairmont/gabs"
	"github.com/stretchr/testify/assert"
)

var id string
var rm *RequestManager

func TestNewRequestManager(t *testing.T) {
	rm = NewRequestManager()
	assert.NotNil(t, rm, "Shouldn't be Nil")
}

func TestCreateRequest(t *testing.T) {
	req := rm.CreateRequest(commandprocess.CallTypeWork, "test", nil, &gabs.Container{}, "appid", "key")
	assert.NotNil(t, req, "Shouldn't be Nil")
	id = req.ID
	assert.NotEmpty(t, id, "Shouldn't be empty id")
}

func TestCount(t *testing.T) {
	crm := NewRequestManager()
	req := crm.CreateRequest(commandprocess.CallTypeWork, "test", nil, &gabs.Container{}, "appid", "key")
	req = crm.CreateRequest(commandprocess.CallTypeWork, "test", nil, &gabs.Container{}, "appid", "key")
	_ = crm.GetRequest(req.ID) //force channel sync
	req.SetComplete()          //make sureit counts completed processes
	assert.Equal(t, crm.Count(), 2, "Should be 2 requests")
}

func TestGetRequest(t *testing.T) {
	r2 := rm.GetRequest(id)
	assert.NotNil(t, r2, "Shouldn't be Nil")
}

func TestRemoveRequest(t *testing.T) {
	rm.RemoveRequest(id)
	r2 := rm.GetRequest(id)
	assert.Nil(t, r2, "Should be Nil")
}

func TestExpireCompletedRequests(t *testing.T) {
	rm.CreateRequest(commandprocess.CallTypeWork, "test1", nil, &gabs.Container{}, "appid", "key").SetComplete()
	rm.CreateRequest(commandprocess.CallTypeWork, "test2", nil, &gabs.Container{}, "appid", "key").SetComplete()

	d, _ := time.ParseDuration("0s")
	expired := rm.ExpireCompletedRequests(d)
	assert.Exactly(t, 2, len(expired), "Should be 2 expired requests")
}

func TestFullJSON(t *testing.T) {
	cp := rm.CreateRequest(commandprocess.CallTypeWork, "test", nil, &gabs.Container{}, "appid", "key")
	_ = rm.GetRequest(cp.ID)
	reqs, err := rm.FullJSON()
	assert.Nil(t, err, "Should be Nil")
	assert.Contains(t, reqs, `"completeTime":`, "Should contain completeTime param")
}

func TestStatusJSON(t *testing.T) {
	rm.CreateRequest(commandprocess.CallTypeWork, "test", nil, &gabs.Container{}, "appid", "key")
	reqs, err := rm.StatusJSON()
	assert.Nil(t, err, "Should be Nil")
	assert.Contains(t, reqs, `"id":`, "Should contain id param")
}
