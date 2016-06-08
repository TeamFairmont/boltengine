// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package requestmanager handles storage and tracking of incoming and ongoing requests
package requestmanager

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/TeamFairmont/boltengine/commandprocess"
	"github.com/TeamFairmont/boltshared/config"
	"github.com/TeamFairmont/gabs"
)

// RequestManager for use tracking CommandProcess
type RequestManager struct {
	requests       map[string]*commandprocess.CommandProcess
	newRequestChan chan newRequest
	getRequestChan chan getRequest
	delRequestChan chan string
	mutex          sync.RWMutex
}

// newRequest is an internal type used to pass to the goroutine a new commandprocess
type newRequest struct {
	cp *commandprocess.CommandProcess
	//returnChan chan int
}

// getRequest is an internal type used to pass to the goroutine a request ID to return
type getRequest struct {
	id         string
	returnChan chan *commandprocess.CommandProcess
}

// NewRequestManager inits a manager for use tracking CommandProcess
func NewRequestManager() *RequestManager {
	rm := RequestManager{}
	rm.requests = make(map[string]*commandprocess.CommandProcess)
	rm.newRequestChan = make(chan newRequest)
	rm.getRequestChan = make(chan getRequest)
	rm.delRequestChan = make(chan string) //should this have a buffer & config var ?

	go func() {
		for {
			select {
			case nr := <-rm.newRequestChan:
				rm.mutex.Lock()
				rm.requests[nr.cp.ID] = nr.cp
				rm.mutex.Unlock()
				//nr.returnChan <- 0
			case gr := <-rm.getRequestChan:
				rm.mutex.RLock()
				gr.returnChan <- rm.requests[gr.id]
				rm.mutex.RUnlock()
			case id := <-rm.delRequestChan:
				rm.mutex.Lock()
				delete(rm.requests, id)
				rm.mutex.Unlock()
			}
		}
	}()

	return &rm
}

// Count returns total number of requests still in memory
func (rm *RequestManager) Count() int {
	return len(rm.requests)
}

// CreateRequest makes a request and logs it in the request manager for later use
// Returns the created CommandProcess instance
func (rm *RequestManager) CreateRequest(reqtype int, cmd string, apicall *config.APICall, payload *gabs.Container, appID, requestKey string) *commandprocess.CommandProcess {
	cp := commandprocess.NewCommandProcess(reqtype, cmd, apicall, payload, appID, requestKey)
	req := newRequest{cp}
	rm.newRequestChan <- req
	//<-req.returnChan
	//id := <-req.returnChan
	return cp
}

// GetRequest Looks up a known request by UUID. Returns nil if not found
func (rm *RequestManager) GetRequest(id string) *commandprocess.CommandProcess {
	gr := getRequest{id, make(chan *commandprocess.CommandProcess)}
	rm.getRequestChan <- gr
	return <-gr.returnChan
}

// RemoveRequest removes a known request by UUID
func (rm *RequestManager) RemoveRequest(id string) {
	rm.delRequestChan <- id
}

// ExpireCompletedRequests loops through all requests that have completed and expires the request if the timeout is reached
// It is intended to be used as part of a go routine that loops every X seconds
func (rm *RequestManager) ExpireCompletedRequests(completeResultExpiration time.Duration) []string {
	expired := []string{}
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	for key, value := range rm.requests {
		value.Mutex.RLock()
		if value.Complete && value.CompleteTime.Add(completeResultExpiration).UnixNano() <= time.Now().UnixNano() {
			expired = append(expired, value.ID)
			rm.mutex.RUnlock()
			rm.RemoveRequest(key)
			rm.mutex.RLock()
		}
		value.Mutex.RUnlock()
	}
	return expired
}

// FullJSON returns a json object in string format containing information about all current requests
func (rm *RequestManager) FullJSON() (string, error) {
	reqs, err := json.MarshalIndent(rm.requests, "", "\t")
	return string(reqs), err
}

// StatusJSON returns a json object in string format containing id and complete status about all current requests
func (rm *RequestManager) StatusJSON() (string, error) {
	type T struct {
		ID        string    `json:"id"`
		Complete  bool      `json:"complete"`
		ReqTime   time.Time `json:"reqTime"`
		HMACGroup string    `json:"group"`
	}
	reqtmp := []T{}

	rm.mutex.RLock()
	for _, v := range rm.requests {
		v.Mutex.Lock()
		reqtmp = append(reqtmp, T{v.ID, v.Complete, v.ReqTime, v.HMACGroup})
		v.Mutex.Unlock()
	}
	rm.mutex.RUnlock()

	reqs, err := json.MarshalIndent(reqtmp, "", "\t")
	return string(reqs), err
}
