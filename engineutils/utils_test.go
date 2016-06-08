// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package engineutils

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetIP(t *testing.T) {
	r, _ := http.NewRequest("GET", "/pending", strings.NewReader(""))
	r.RemoteAddr = "9.8.7.6:45678"
	ip := GetIP(r)
	assert.Contains(t, ip, "9.8.7.6", "Ip should be 9.8.7.6")
}
