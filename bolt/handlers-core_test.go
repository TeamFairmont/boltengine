// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TeamFairmont/boltengine/requestmanager"
	"github.com/stretchr/testify/assert"
)

var ctx *Context

func TestCoreHandleTest(t *testing.T) {
	//cfgdir := "../etc/bolt/"
	//cfgpath := "../etc/bolt/config.json"
	ctx = &Context{}
	ctx.Engine = CreateTestEngine("error")
	ctx.Engine.Requests = requestmanager.NewRequestManager()

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/test", strings.NewReader(""))

	err := coreHandleTest(ctx, w, r, "")
	assert.Nil(t, err, "err should be nil")
	assert.Equal(t, `{"test": 1}`, w.Body.String(), "Should be equal")
}

func TestCoreHandleEcho(t *testing.T) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/echo/unit", strings.NewReader(""))

	err := coreHandleEcho(ctx, w, r, "")
	assert.Nil(t, err, "err should be nil")
	assert.Equal(t, `{"echo": "/echo/unit"}`, w.Body.String(), "Should be equal")
}

func TestCoreHandleTime(t *testing.T) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/time", strings.NewReader(""))

	err := coreHandleTime(ctx, w, r, "")
	assert.Nil(t, err, "err should be nil")
	assert.Contains(t, w.Body.String(), `"time":`, "Should contain time param")
}

func TestCoreHandleDebugLog(t *testing.T) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/debug-log", strings.NewReader(""))

	err := coreHandleDebugLog(ctx, w, r, "")
	assert.Nil(t, err, "err should be nil")
	assert.Equal(t, w.Code, 200, "Should return 200 OK response")
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"), "Content type should be text/plain")
}

func TestCoreHandleStats(t *testing.T) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/stats", strings.NewReader(""))

	err := coreHandleStats(ctx, w, r, "")
	assert.Nil(t, err, "err should be nil")
	assert.Contains(t, w.Body.String(), `"value":`, "Should contain value param")
}

func TestCoreHandleGetConfig(t *testing.T) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/get-config", strings.NewReader(""))

	err := coreHandleGetConfig(ctx, w, r, "")
	assert.Nil(t, err, "err should be nil")
	assert.Contains(t, w.Body.String(), `"engine":`, "Should contain engine param from config")
}

func TestCoreHandlePending(t *testing.T) {
	cp := ctx.Engine.Requests.CreateRequest(1, "", nil, nil, "", "")
	_ = ctx.Engine.Requests.GetRequest(cp.ID)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/pending", strings.NewReader(""))

	err := coreHandlePending(ctx, w, r, "")
	assert.Nil(t, err, "err should be nil")
	assert.Contains(t, w.Body.String(), "reqTime", "Should contain reqTime")
}
