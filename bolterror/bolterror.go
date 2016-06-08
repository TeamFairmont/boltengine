// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bolterror provides the methods by which an error is defined and formatted for communication back
// to the API caller.
package bolterror

import "github.com/TeamFairmont/gabs"

// ErrType constants for NewBoltError. DO NOT CHANGE THE ORDER!
// Always add a new type at the bottom of the list
const (
	Internal = iota //An error within the engine logic/operation itself
	Request         //An error related to an incoming/in-process API call
	Timeout         //Call or command wasnt completed before the allocated timeout period
	Zombie          //Last command wasn't completed before allocated 'zombie' give-up time. In a healthy system these shouldn't happen
)

// BoltError is the wrapper for an error that needs to be communicated back to the API caller.
// It is recommended that it not be created directly, but via the NewBoltError function
type BoltError struct {
	Srcerr  error
	Parent  string
	Details string
	Input   string
	ErrType int
}

// NewBoltError creates a BoltError struct using an optional source error and other details. The Parent
// parameter is used to differentiate this error from other errors in the event that a response contains multiple
// errors.
func NewBoltError(err error, parent string, details string, input string, errtype int) *BoltError {
	er := BoltError{}
	er.Srcerr = err
	er.Details = details
	er.Input = input
	er.ErrType = errtype
	er.Parent = parent
	return &er
}

// AddToPayload takes a gabs JSON container and adds this error's contents to the container in JSON format.
// The format is:
//
//      "error": {
//          BoltError.Parent {
//              "details": BoltError.Details,
//              "type": BoltError.ErrType,
//              "input": BoltError.Input,
//          }
//      }
//
func (e *BoltError) AddToPayload(payload *gabs.Container) {
	payload.SetP(e.Details, "error."+e.Parent+".details")
	payload.SetP(e.ErrType, "error."+e.Parent+".type")
	payload.SetP(e.Input, "error."+e.Parent+".input")
}
