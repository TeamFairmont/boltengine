// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package utils

// NilString takes in an interface as val, and if its nil, returns ifnil,
// otherwise returns the val cast as a string
func NilString(val interface{}, ifnil string) string {
	if val == nil {
		return ifnil
	}
	return val.(string)
}

// StringInSlice takes a string, and slice of strings, and checks if the
// slice contains it. Returns true if found
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
