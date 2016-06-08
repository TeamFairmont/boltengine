// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolterror

import (
	"testing"

	"github.com/TeamFairmont/gabs"
	"github.com/stretchr/testify/assert"
)

func TestAddToPayload(t *testing.T) {
	er := NewBoltError(nil, "p", "d", "i", Internal)
	con, _ := gabs.ParseJSON([]byte("{\"error\":{\"someOtherError\":{\"details\":\"1\",\"input\":\"2\",\"type\":0}}}"))
	er.AddToPayload(con)

	assert.Equal(t, "{\"error\":{\"p\":{\"details\":\"d\",\"input\":\"i\",\"type\":0},\"someOtherError\":{\"details\":\"1\",\"input\":\"2\",\"type\":0}}}", con.String(), "JSON output matches expected")
}

func TestNewBoltError(t *testing.T) {
	er := NewBoltError(nil, "p", "d", "i", Internal)

	assert.Equal(t, &BoltError{nil, "p", "d", "i", Internal}, er, "BoltError matches expected")
}
