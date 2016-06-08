// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package throttle

import (
	"time"

	"github.com/TeamFairmont/boltshared/config"
)

// InitThrottleGroups takes the security groups and empty throttle map
// It returns the throttle map with individual maps for each group name
// Create a map where the keys are the group's name, and will contain a map of timestamps
func InitThrottleGroups(groups *[]config.SecurityGroups, throttle map[string]map[int]time.Time) map[string]map[int]time.Time {
	// Initialize the Throttle map
	throttle = make(map[string]map[int]time.Time)

	// Loop through the groups and create a key and empty map for each group name
	for _, thisGroup := range *groups {
		throttle[thisGroup.Name] = make(map[int]time.Time)
	}
	return throttle
}

// GetThrottleForGroup takes a group name and a pointer to array of groups.
// It returns an int64 representing the number of requests the group is allowed to make per second, or 0 (do not throttle) if the key doesn't exist.
func GetThrottleForGroup(group string, groups *[]config.SecurityGroups) (requestsPerSecond int64) {
	for _, thisGroup := range *groups {
		if thisGroup.Name == group {
			return thisGroup.RequestsPerSecond
		}
	}
	// Group name not found or the group doesn't have a value for throttleRequestsMs.  Return 0.
	return 0
}

// GroupLimitReached takes the throttle groups, the group name to check, and the requests the group is limited to per second.
// It counts the group's requests made in the past second that have not been throttled.
// Returns true if the group has exceeded their request per second count.  Otherwise, it returns false.
func GroupLimitReached(throttle map[string]map[int]time.Time, groupname string, requestsPerSecond int64) bool {
	// Determine the time 1 second ago for comparison
	timeNow := time.Now()
	oneSecondAgo := timeNow.Add(-1 * time.Second)

	// Loop through this group's timestamps, creating a new map of fresh timestamps (no older than the past second)
	var freshMap map[int]time.Time
	freshMap = make(map[int]time.Time)
	freshCount := 0
	for index := range throttle[groupname] {
		if oneSecondAgo.Before(throttle[groupname][index]) {
			freshMap[freshCount] = throttle[groupname][index]
			freshCount++
		}
	}
	// Overwrite the existing map with the new map
	throttle[groupname] = freshMap

	// If the group has not hit their requests-per-second limit- Record the current request's timestamp and return false.
	if int64(len(throttle[groupname])) < requestsPerSecond {
		addIndex := len(throttle[groupname])
		throttle[groupname][addIndex] = time.Now()
		return false
	}
	// Otherwise, the group has exceeded their throttle limit, return true.
	// No need to record the current request's timestamp because the request will be rejected.
	return true
}
