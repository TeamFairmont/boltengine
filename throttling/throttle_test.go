// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package throttle

import (
	"log"
	"testing"
	"time"

	"github.com/TeamFairmont/boltshared/config"
	"github.com/TeamFairmont/gabs"
	"github.com/stretchr/testify/assert"
)

type Engine struct {
	Config   *config.Config
	Throttle map[string]map[int]time.Time
}

func TestMatches(tst *testing.T) {
	var engine *Engine

	// Parse the config file's json.
	configjsonParsed, err := gabs.ParseJSON([]byte(`{
    "security": {
        "verifyTimeout": 30,
        "groups": [{
            "name": "group_with_throttle",
            "hmackey": "key_goes_here",
            "requestsPerSecond": 3
        }, {
            "name": "group_without_throttle",
            "hmackey": "key_goes_here"
        }]
    }
	}`))
	if err != nil {
		log.Fatal("Error parsing json:\n", err, "\n")
	}

	// Create a customized config
	cfg, err := config.DefaultConfig()
	assert.Nil(tst, err, "No error")
	cfg, err = config.CustomizeConfig(cfg, configjsonParsed.String())
	assert.Nil(tst, err, "No error")

	//Initialize throttling
	engine = &Engine{}
	engine.Config = cfg
	engine.Throttle = InitThrottleGroups(&engine.Config.Security.Groups, engine.Throttle)
	// fmt.Println(engine.Throttle)

	// Test getting RPS for a group with a throttle value
	requestsPerSecond := GetThrottleForGroup("group_with_throttle", &engine.Config.Security.Groups)
	assert.Equal(tst, int64(3), requestsPerSecond, "RPS for group_with_throttle should be 3 (of type int64)")

	// Test getting RPS for a group without a throttle value (should return 0)
	requestsPerSecond = GetThrottleForGroup("group_without_throttle", &engine.Config.Security.Groups)
	assert.Equal(tst, int64(0), requestsPerSecond, "RPS for group_without_throttle should be 0 (of type int64)")

	// Test throttle limiting by sending multiple requests rapidly.
	// The first 3 requests in less than 1 second should not return false (not throttled)
	// The 4th request in less than 1 second should return true (throttled)
	// After pausing for a second and attempting another request, it should return false.
	requestsPerSecond = int64(3)
	// Check 1
	limitReached := GroupLimitReached(engine.Throttle, "group_with_throttle", requestsPerSecond)
	assert.Equal(tst, false, limitReached, "GroupLimitReached check 1 should return false.")
	// Check 2
	limitReached = GroupLimitReached(engine.Throttle, "group_with_throttle", requestsPerSecond)
	assert.Equal(tst, false, limitReached, "GroupLimitReached check 2 should return false.")
	// Check 3
	limitReached = GroupLimitReached(engine.Throttle, "group_with_throttle", requestsPerSecond)
	assert.Equal(tst, false, limitReached, "GroupLimitReached check 3 should return false.")
	// Check 4
	limitReached = GroupLimitReached(engine.Throttle, "group_with_throttle", requestsPerSecond)
	assert.Equal(tst, true, limitReached, "GroupLimitReached check 4 should return true (limit reached).")
	// Check 5
	limitReached = GroupLimitReached(engine.Throttle, "group_with_throttle", requestsPerSecond)
	assert.Equal(tst, true, limitReached, "GroupLimitReached check 5 should return true (limit reached).")

	// Pause for a second, then try again for the group with throttling who previous hit their limit.
	duration := time.Duration(1) * time.Second
	time.Sleep(duration)
	limitReached = GroupLimitReached(engine.Throttle, "group_with_throttle", requestsPerSecond)
	assert.Equal(tst, false, limitReached, "GroupLimitReached check 6 (after pause) should return false.")

}
