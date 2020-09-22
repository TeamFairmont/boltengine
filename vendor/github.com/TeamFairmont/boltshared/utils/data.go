// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package utils

import (
	"errors"

	"github.com/TeamFairmont/gabs"
)

var (
	done          = make(chan bool)
	boltIteration int //holds the current bolt iteration number, for shutting down go routines for reboot
	resDoneChan   = make(chan bool)
)

// SignalResDoneChan sends the done signal to continue the reboot process
func SignalResDoneChan() {
	resDoneChan <- true
}

// GetResDoneChan gets the channel
func GetResDoneChan() chan bool {
	return resDoneChan
}

// GetDoneChannel returns the done channel, for use in other packages
func GetDoneChannel() chan bool {
	return done
}

// InitDoneChan reinnitialized done channel
func InitDoneChan() {
	done = make(chan bool)
}

// CloseDoneChan closes the channel
func CloseDoneChan() {
	close(done)
}

//CheckBoltIteration checks to see if the current boltIteration matches the go routines boltIteration, if not close the go routines
func CheckBoltIteration(goRoutineBoltIteration int) (bool, error) {
	switch {
	case goRoutineBoltIteration != boltIteration:
		{
			if boltIteration < goRoutineBoltIteration {
				return false, errors.New("Bolt Iteration mismatch, boltIteration is too small")
			}
			return false, nil

		}
	default:
		{ //iterations match
			return true, nil
		}
	}
}

//GetBoltIteration returns the current bolt iteration
func GetBoltIteration() int {
	return boltIteration
}

//InitBoltIteration initializes the variable that holds the bolt engines current itterations
func InitBoltIteration() {
	boltIteration = 0
}

//IncrementBoltIteration increments boltIteration to hold the current number of bolt itterations
//for closing go routines on reboot
func IncrementBoltIteration() {
	boltIteration++
}

// FilterPayload recieves a gabs.Container and checks it against []keys, an array of strings.
// Any top-level children of the payload that aren't in []keys are removed. Returns the modified gabs.Container
func FilterPayload(p *gabs.Container, keys []string) (*gabs.Container, error) {
	//copy payload, so origional is not altered
	var payload = gabs.New()
	var err error
	if p != nil {
		payload, err = gabs.ParseJSON([]byte(p.String()))
	} else {
		payload, err = gabs.ParseJSON([]byte(`{}`))
		return nil, err
	}

	//uses gabs children function and checks for an error.
	children, err := payload.ChildrenMap()
	if err != nil {
		return payload, err
	}
	//Loops through every child in children
	//test every key on every child to see if they match in utils.StringInSlice
	//if no match is found, the child is deleted
	if len(keys) >= 1 {
		for child := range children {
			if !StringInSlice(child, keys) {
				payload.Delete(child)
			}
		}
	}
	return payload, err
}
