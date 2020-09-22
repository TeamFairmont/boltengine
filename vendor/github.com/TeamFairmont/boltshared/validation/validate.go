// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package validate

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/TeamFairmont/gabs"
)

var lock = sync.RWMutex{}

//isNumber determines if a datatype is a float64.
//This is used to avoid panics when validating a requiredType of int64 or float64 and the received type is non-numeric.
// Because gabs uses the standard golang JSON package for parsing bytes, all numbers are parsed into type float64, regardless of whether they're floating point or not.
// https://github.com/Jeffail/gabs/issues/12
func isNumber(numberIn interface{}) bool {
	switch numberIn.(type) {
	case float64:
		return true
	default:
		// The data type is not a number
		return false
	}
}

//matches takes a variable of any type and a string value describing the expected type.
//If the variable matches the expected type, return true.
func matches(paramvalue interface{}, requiredType string) bool {

	switch requiredType {
	// The default numeric type is float64.  If the required type is another numeric type, verify the value received can be converted to required type.
	case "int64":
		//To avoid a panic, non-float64 numeric types must first be checked to make sure the received type is indeed a number.
		if !isNumber(paramvalue) {
			return false
		}
		// Attempt to parse an int64 from the float64
		v := paramvalue.(float64)
		vf := strconv.FormatFloat(v, 'f', -1, 64)
		strToInt, err := strconv.ParseInt(vf, 10, 64)
		if err != nil {
			// This parameter's number can't be converted to an int64
			return false
		}
		paramvalue = strToInt

		// Add more type checking as needed by inserting more cases in this switch.

	}

	// Verify that the required type matches the data type of the payload's value.
	// If a required numeric type is not float64, the payload must be converted in the outer switch above.
	reflectType := reflect.TypeOf(paramvalue).String()
	if reflectType == requiredType {
		return true
	}
	return false

}

//CheckPayloadReqParams loops through the required params and confirms they exist in the payload and are the correct type.
func CheckPayloadReqParams(requiredParams map[string]string, payload *gabs.Container) error {

	// For validation, loop through the required params and confirm they exist in the payload and are the correct data type
	// If the requiredparam doesn't have a matching payload value with the correct data type, log an error and return
	for reqkey, reqtype := range requiredParams {
		keydata := payload.Path("initial_input").Path(reqkey).Data()
		// Determine the payload contains this key
		if keydata == nil {
			return errors.New("Missing parameter: " + reqkey)
		}

		// Verify the payload value for this key is the required type
		if !matches(keydata, reqtype) {
			errorString := []string{"Parameter:", reqkey, ", Expected:", reqtype, ", Received:", reflect.TypeOf(keydata).String()}
			return errors.New(strings.Join(errorString, ""))
		}

	}

	return nil
}

// CheckPayloadStructure checks that a payload has all top-level fields necessary to be processed
// by the bolt engine
func CheckPayloadStructure(payload *gabs.Container) error {
	/*"initial_input":{},
	  "return_value":{},
	  "data":{},
	  "trace":[],
	  "debug":{},
	  "nextCommand":"",
	  "error":{},
	  "config":{},
	  "params":{}*/
	keys := []string{"initial_input", "return_value", "data", "trace", "debug", "nextCommand", "error", "config", "params"}

	for k := range keys {
		lock.RLock()
		defer lock.RUnlock()
		if !payload.Exists(keys[k]) {
			return errors.New("Payload missing " + keys[k])
		}
	}
	return nil
}
