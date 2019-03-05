// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package commandprocess handles manipulation of an actual in-process API call.
// It sets up defaults, stores basic performance info and current command state.
package commandprocess

// TODO(r): peektime to be used to detect 'hung' jobs. if a no-timeout job takes > peekTime+hungProcTime it'll be removed and the goroutine force closed

import (
	"sync"
	"time"

	"github.com/TeamFairmont/boltshared/config"
	"github.com/TeamFairmont/gabs"
	"github.com/TeamFairmont/go.uuid"
)

// API call types.
const (
	CallTypeWork    = iota // Work is "fire and forget" with no way for the caller to get the result.
	CallTypeTask           // Task is "fire and check" where the call returns the id immediately, the caller checks in later to get the result
	CallTypeRequest        // Request is "fire and wait" where the caller won't receive a response until the command completes or times out.
)

// EmptyPayload stores the base JSON string for a worker payload
const EmptyPayload = `{
	"initial_input":{},
	"return_value":{},
	"data":{},
	"trace":[],
	"debug":{},
	"nextCommand":"",
	"error":{},
	"config":{},
	"params":{}
}`

// CommandProcess stores in-process API call information
type CommandProcess struct {
	ID                 string          `json:"id"`      //UUID for this request
	HMACGroup          string          `json:"group"`   //The caller's API group name
	HMACToken          string          `json:"-"`       //The caller's API HMAC token (not key)
	Payload            *gabs.Container `json:"-"`       //Payload sent/recv with workers. Contains json format from static/emptyPayload.json
	InitialInputString string          `json:"-"`       //Copy of initial input in json string form from the payload when the CommandProcess was initialized
	InitialCommand     string          `json:"apiCall"` //The first command to be run in this call
	APICall            *config.APICall `json:"-"`
	CallType           int             `json:"callType"`  //Value from one of the CallType constants
	ReqTime            time.Time       `json:"reqTime"`   //The timestamp this request was first created
	PeekTime           time.Time       `json:"peekTime"`  //PeekTime is updated whenever a client requests a peek into this calls status. It is used in the algo to detect hung calls
	PeekCount          int             `json:"peekCount"` //PeekCount is number of times the call has been Peek'ed

	Complete     bool      `json:"complete"`     //True if all commands have completed.
	CompleteTime time.Time `json:"completeTime"` //Timestamp of when 'completed' was set true

	CommandTime         time.Time           `json:"lastCommandTime"` //The timestamp of the last command entered into MQ
	LastPrimaryCommand  string              `json:"lastCommand"`     //The config-based subcommand called (doesnt update for 'nextCommand' overrides)
	CurrentCommand      *config.CommandInfo `json:"-"`               //Info struct for the currently executing command
	CurrentCommandIndex int                 `json:"-"`               //Array index of current command
	NextCommand         string              `json:"nextCommand"`     //If this is set by a worker in the payload, this command will be executed before executing the next config-based command

	TimeoutChannel chan bool `json:"-"` //When StartTimeout() is called, this is set to the timeout channel
	TimeoutStarted bool      `json:"-"` //When StartTimeout() is called, this is set to true

	Mutex sync.RWMutex `json:"-"`
}

// NewCommandProcessWithID creates a command process instance, sets defaults, etc, and allows the caller to specify the uuid
func NewCommandProcessWithID(id string, calltype int, cmd string, apicall *config.APICall, payload *gabs.Container, hmacgroup, hmactoken string) *CommandProcess {
	cp := CommandProcess{}
	cp.ID = id
	cp.CallType = calltype
	cp.InitialCommand = cmd
	cp.APICall = apicall
	cp.Payload = payload
	cp.ReqTime = time.Now()
	cp.HMACGroup = hmacgroup
	cp.HMACToken = hmactoken
	cp.PeekTime = cp.ReqTime

	return &cp
}

// NewCommandProcess creates a command process instance, sets defaults, etc, and generates a new uuid
func NewCommandProcess(calltype int, cmd string, apicall *config.APICall, payload *gabs.Container, appID string, requestKey string) *CommandProcess {
	cp := NewCommandProcessWithID(uuid.NewV4().String(), calltype, cmd, apicall, payload, appID, requestKey)
	return cp
}

// SetInitialInput sets the intiial payload input and stores it in string form for future use in case of changes
func (cp *CommandProcess) SetInitialInput(input *gabs.Container) {
	cp.Payload.SetP(input.Data(), "initial_input")
	cp.InitialInputString = input.String()
}

// StartTimeout creates a channel and starts the timeout goroutine for this process
func (cp *CommandProcess) StartTimeout() chan bool {
	cp.TimeoutStarted = true
	timeout := make(chan bool, 1)
	if cp.APICall.ResultTimeout > 0 {
		go func() { //seems reboot safe
			time.Sleep(cp.APICall.ResultTimeout)
			timeout <- true
		}()
	}
	cp.TimeoutChannel = timeout
	return timeout
}

// UpdatePeekTime sets the requests peek time to Now
func (cp *CommandProcess) UpdatePeekTime() {
	cp.Mutex.Lock()
	defer cp.Mutex.Unlock()
	cp.PeekTime = time.Now()
	cp.PeekCount++
}

// SetComplete sets the Complete flag to true, CompleteTime
func (cp *CommandProcess) SetComplete() {
	cp.Mutex.Lock()
	defer cp.Mutex.Unlock()
	cp.Complete = true
	cp.CompleteTime = time.Now()
}

// AddTraceEntry copies a snapshot of relevant Payload fields into the Payload's trace array
func (cp *CommandProcess) AddTraceEntry() {
	cp.Mutex.Lock()
	defer cp.Mutex.Unlock()

	trace, _ := gabs.ParseJSON([]byte("{}"))
	trace.SetP(cp.Payload.Path("return_value").Data(), "return_value")
	trace.SetP(cp.Payload.Path("data").Data(), "data")
	trace.SetP(cp.Payload.Path("config").Data(), "config")
	trace.SetP(cp.Payload.Path("params").Data(), "params")
	if cp.CurrentCommand != nil {
		trace.SetP(cp.CurrentCommand.Name, "command")
	}
	trace.SetP(cp.CurrentCommandIndex, "commandIndex")
	trace.SetP(time.Now(), "timestamp")
	cp.Payload.ArrayAppendP(trace.Data(), "trace")
}
