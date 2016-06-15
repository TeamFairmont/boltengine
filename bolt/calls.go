// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/TeamFairmont/amqp"
	"github.com/TeamFairmont/boltengine/bolterror"
	"github.com/TeamFairmont/boltengine/commandprocess"
	"github.com/TeamFairmont/boltengine/engineutils"
	"github.com/TeamFairmont/boltshared/mqwrapper"
	"github.com/TeamFairmont/boltshared/validation"
	"github.com/TeamFairmont/gabs"
)

// HandleCall preps and performs queing, validation, and initiates processing for an API call.
// reqtype is a constant which tells the engine how to handle the request (commandprocess.CallType*).
//		CallTypeRequest: Engine processes the API call and returns the result, however if the timeout is reached,
//					 it will return the task ID so the client can retrieve the result later
// 		CallTypeTask: Engine processes the API call, but returns the task ID immedately without waiting for the result.
//		CallTypeWork: Engine processes the API call with no task ID or returned results
func (engine *Engine) HandleCall(reqtype int, w http.ResponseWriter, r *http.Request, hmacGroup string) {
	if engine.IsShutdown() {
		ret := gabs.New()
		bolterror.NewBoltError(errors.New(""), "shutdown", "Engine restarting, please try again", "", bolterror.Internal).AddToPayload(ret)
		fmt.Fprint(w, ret.String())
		return
	}

	if engine.mqConnection == nil {
		ret := gabs.New()
		bolterror.NewBoltError(errors.New(""), "maintenance", "Engine temporarily unavailable, please try again", "", bolterror.Internal).AddToPayload(ret)
		fmt.Fprint(w, ret.String())
		return
	}

	if strings.ToUpper(r.Method) == "POST" {
		//call extract and validation
		cmd, payload, err := callVars(r)
		if err != nil {
			payloadstr := ""
			if payload != nil {
				payloadstr = payload.String()
			}
			engine.LogDebug("request_malformed", logrus.Fields{"cmd": cmd, "payloadstr": payloadstr, "ip": engineutils.GetIP(r)}, "Malformed request")
			engine.OutputError(w, bolterror.NewBoltError(err, "request", "Malformed request. API call name or payload could not be processed", cmd+":"+payloadstr, bolterror.Request))
			return
		}

		//pull apicall struct
		apicall, ok := engine.Config.APICalls[cmd]
		if !ok {
			engine.LogDebug("request_unknown_apicall", logrus.Fields{"cmd": cmd, "ip": engineutils.GetIP(r)}, "Unknown API Call")
			engine.OutputError(w, bolterror.NewBoltError(err, "request", "Unknown API Call", cmd, bolterror.Request))
			return
		}

		//create request
		secgroup := hmacGroup
		token := "" // dont set this
		fullPayload, _ := gabs.ParseJSON([]byte(commandprocess.EmptyPayload))

		req := engine.Requests.CreateRequest(reqtype, cmd, &apicall, fullPayload, secgroup, token)

		// Timestamp the request
		req.Payload.SetP(time.Now(), "call_in")
		req.SetInitialInput(payload)

		req.Payload.SetP(req.ID, "id")
		engine.LogInfo("call_in", logrus.Fields{"id": req.ID, "url": r.URL.String(), "payload": payload}, "Api call in")
		engine.Stats.Ch("performance").Ch("calls").Ch(req.InitialCommand).Ch("hits").Incr()

		noCache := true
		//if BOLT-NO-CACHE does not exist, continue normally
		_, exist := r.Header["Bolt-No-Cache"] //keys cases change. BOLT-NO-CACHE changes to Bolt-No-Cache
		if !exist {
			cacheval, err := engine.GetCacheItem(cmd, payload.String())

			if err == nil {
				returnvalue, err := gabs.ParseJSON([]byte(cacheval))
				if err != nil {
					engine.DelCacheItem(cmd, payload.String())
					engine.LogWarn("cache_error", logrus.Fields{"id": req.ID, "command": req.InitialCommand, "cached": true}, "Cached value couldn't be parsed to JSON")
				} else {
					noCache = false
					req.SetComplete()
					req.Payload.SetP(req.Complete, "complete")
					req.Payload.SetP(returnvalue.Data(), "return_value")
					req.Payload.SetP(true, "cached")

					//really you shouldnt have 'work'-able api calls be cachable as well, but just in case, keep consistent output
					if reqtype == commandprocess.CallTypeWork {
						ret := gabs.New()
						ret.SetP(nil, "id")
						fmt.Fprint(w, ret.String())
					} else {
						//OutputRequst takes w http.ResponseWritter, req *commandprocess.CommandProcess and APICall.FilterKeys []string
						engine.OutputRequest(w, req, apicall.FilterKeys)
					}
				}
			}
			if !noCache {
				engine.Stats.Ch("general").Ch("cache_hits").Incr()
				engine.Requests.RemoveRequest(req.ID)
				engine.LogInfo("call_out", logrus.Fields{"id": req.ID, "command": req.InitialCommand, "cached": true}, "Api call out")
			}
		} else { //BOLT-NO-CACHE did exist, so increment it.
			engine.Stats.Ch("general").Ch("cache_overrides").Incr()
		}

		if noCache {
			if engine.Config.Cache.Type != "" {
				engine.Stats.Ch("general").Ch("cache_misses").Incr()
			}

			switch reqtype {
			case commandprocess.CallTypeRequest:
				engine.processCall(req)

				req.Mutex.Lock()
				req.Payload.SetP(req.Complete, "complete")

				//OutputRequst takes w http.ResponseWritter, req *commandprocess.CommandProcess and APICall.FilterKeys []string
				engine.OutputRequest(w, req, apicall.FilterKeys)
				engine.LogInfo("call_out", logrus.Fields{"id": req.ID, "command": req.InitialCommand}, "Api call out")
				if req.Complete {
					engine.Requests.RemoveRequest(req.ID)
				}
				req.Mutex.Unlock()

			case commandprocess.CallTypeTask:
				go engine.processCall(req)
				ret := gabs.New()
				ret.SetP(req.ID, "id")
				fmt.Fprint(w, ret.String())

			case commandprocess.CallTypeWork:
				go engine.processCall(req)
				ret := gabs.New()
				ret.SetP(nil, "id")
				fmt.Fprint(w, ret.String())

			}
		}
	}
}

// callVars pulls the json input and api command name from the http.request struct
func callVars(r *http.Request) (call string, payload *gabs.Container, err error) {
	call, _ = ExtractCallName(r)
	jsonstr, _ := ioutil.ReadAll(r.Body)
	jsobj, err := gabs.ParseJSON([]byte(jsonstr))
	payload = jsobj
	return
}

// processCall performs initial validation, sets up, and executes an api call's commands via processCommands()
func (engine *Engine) processCall(proc *commandprocess.CommandProcess) {

	engine.LogDebug("processCall reqparams", logrus.Fields{"requiredparams": proc.APICall.RequiredParams}, "")
	err := validate.CheckPayloadReqParams(proc.APICall.RequiredParams, proc.Payload)
	if err != nil {
		bolterror.NewBoltError(err, "validation", "Error validating required parameters", proc.InitialCommand, bolterror.Request).AddToPayload(proc.Payload)
		engine.LogInfo("validation", logrus.Fields{"err": err, "id": proc.ID, "apiCall": proc.InitialCommand}, "Error validating required parameters")
		proc.SetComplete()
		return
	}

	//empty command call just returns completed
	if len(proc.APICall.Commands) == 0 {
		proc.SetComplete()
		return
	}

	//setup initial command to start for this call
	proc.CurrentCommand = &proc.APICall.Commands[0]
	proc.CurrentCommandIndex = 0

	//setup 'global' worker config
	if engine.Config.WorkerConfigObj != nil && engine.Config.WorkerConfigObj.Data() != nil {
		proc.Payload.SetP(engine.Config.WorkerConfigObj.Data(), "config")
	}

	//setup mq for this call, try to create a channel just for this call, if it fails fall back to the general channel

	ch, err := engine.mqConnection.Connection.Channel()
	if err != nil {
		engine.LogError("mq_error", nil, err.Error())
		ch = engine.mqConnection.Channel
	} else {
		ch.Qos(1, 0, false)
	}

	//ch := engine.mqConnection.Channel
	q, res, err := mqwrapper.CreateConsumeTempQueue(ch)

	if err != nil {
		proc.SetComplete()
		engine.LogError("mq_error", nil, err.Error())
		bolterror.NewBoltError(err, "mq", "Error creating MQ queue", proc.InitialCommand, bolterror.Internal).AddToPayload(proc.Payload)
		return
	}

	engine.processCommands(proc, res, ch, q, false, false)
}

// processCommands starts and continues pushing commands to the mq, assembling and validating the payload at each step
// also watches for timeouts due to call, command, or zombie and calls itself to continue processing after timeout if applicable.
func (engine *Engine) processCommands(proc *commandprocess.CommandProcess, res <-chan amqp.Delivery, ch *amqp.Channel, q *amqp.Queue, skipInitialCommand bool, skipTimeouts bool) {

	err := validate.CheckPayloadStructure(proc.Payload)
	if err != nil {
		bolterror.NewBoltError(err, "validation", "Error validating required parameters", proc.InitialCommand, bolterror.Request).AddToPayload(proc.Payload)
		engine.LogInfo("validation", logrus.Fields{"err": err, "id": proc.ID, "command": proc.CurrentCommand.Name}, "Error validating payload structure")
		proc.SetComplete()
		return
	}

	engine.LogDebug("processCommands reqparams", logrus.Fields{"currentcommand": proc.CurrentCommand.Name, "requiredParams": engine.Config.CommandMetas[proc.CurrentCommand.Name].RequiredParams}, "")
	err = validate.CheckPayloadReqParams(engine.Config.CommandMetas[proc.CurrentCommand.Name].RequiredParams, proc.Payload)
	if err != nil {
		bolterror.NewBoltError(err, "validation", "Error validating required parameters", proc.InitialCommand, bolterror.Request).AddToPayload(proc.Payload)
		engine.LogInfo("validation", logrus.Fields{"err": err, "id": proc.ID, "command": proc.CurrentCommand.Name}, "Error validating required parameters")
		proc.SetComplete()
		return
	}

	if !proc.TimeoutStarted {
		proc.StartTimeout()
	}

	//make initial subcommand request to mq
	if !skipInitialCommand {
		if proc.NextCommand != "" {
			engine.LogDebug("cmd_queued_next", logrus.Fields{"id": proc.ID, "nextCommand": proc.NextCommand}, "")
			nexttmp := proc.NextCommand
			if engine.Config.Engine.TraceEnabled {
				proc.AddTraceEntry()
			}
			proc.Mutex.Lock()
			proc.NextCommand = ""
			err = mqwrapper.PublishCommand(engine.mqConnection.Channel, proc.ID, engine.Config.Engine.Advanced.QueuePrefix, nexttmp, proc.Payload, q.Name)
			proc.CommandTime = time.Now()
			proc.Mutex.Unlock()
		} else {
			engine.LogDebug("cmd_queued", logrus.Fields{"id": proc.ID, "command": proc.CurrentCommand.Name}, "")

			proc.Mutex.Lock()
			proc.Payload.SetP(proc.CurrentCommand.ConfigParamsObj.Data(), "params")
			proc.Mutex.Unlock()

			if engine.Config.Engine.TraceEnabled {
				proc.AddTraceEntry()
			}
			proc.Mutex.Lock()
			err = mqwrapper.PublishCommand(engine.mqConnection.Channel, proc.ID, engine.Config.Engine.Advanced.QueuePrefix, proc.CurrentCommand.Name, proc.Payload, q.Name)
			proc.CommandTime = time.Now()
			proc.Mutex.Unlock()
		}
	}

	//loop and process subcommands, validate results+params, etc
	stop := false
	for !stop {

		if err != nil {
			bolterror.NewBoltError(err, "request", "Internal error: "+err.Error(), proc.CurrentCommand.Name, bolterror.Internal).AddToPayload(proc.Payload)
			engine.completeProcess(proc, ch, q)
			return
		}

		//command-level timeout
		timeout := make(chan bool, 1)
		if !skipTimeouts && proc.CurrentCommand.ResultTimeout > 0 {
			to := proc.CurrentCommand.ResultTimeout
			go func(to time.Duration) {
				time.Sleep(to)
				timeout <- true
			}(to)
		}

		//zombie cancel further processing
		zombie := make(chan bool, 1)
		if proc.APICall.ResultZombie > 0 {
			to := proc.APICall.ResultZombie
			go func(to time.Duration) {
				time.Sleep(to)
				zombie <- true
			}(to)
		}

		var d amqp.Delivery

		select {
		case <-zombie:
			engine.LogWarn("call_zombie", logrus.Fields{"id": proc.ID, "command": proc.CurrentCommand.Name}, proc.InitialCommand)
			engine.Stats.Ch("calls").Ch(proc.InitialCommand).Ch("zombie_count").Incr()
			bolterror.NewBoltError(nil, "zombie", "API Call zombie time limit reached, retry request and contact sysadmin if issue persists", proc.CurrentCommand.Name, bolterror.Zombie).AddToPayload(proc.Payload)
			engine.completeProcess(proc, ch, q)
			return

		case <-timeout:
			//note: timeout "errors" don't carry over into the final result, if commands continue to sucessfully process
			engine.LogInfo("command_timeout", logrus.Fields{"id": proc.ID, "command": proc.CurrentCommand.Name}, proc.InitialCommand)
			engine.Stats.Ch("calls").Ch(proc.InitialCommand).Ch("command_timeouts").Incr()
			engine.Stats.Ch("commands").Ch(proc.CurrentCommand.Name).Ch("timeouts").Incr()
			bolterror.NewBoltError(nil, "timeout", "Command timeout, use id to fetch result", proc.CurrentCommand.Name, bolterror.Timeout).AddToPayload(proc.Payload)
			go engine.processCommands(proc, res, ch, q, true, true) //doesn't skip the current command object pushing to mq before waiting on the channel
			return

		case <-proc.TimeoutChannel:
			//note: timeout "errors" don't carry over into the final result, if commands continue to sucessfully process
			engine.LogInfo("call_timeout", logrus.Fields{"id": proc.ID, "command": proc.CurrentCommand.Name}, proc.InitialCommand)
			engine.Stats.Ch("calls").Ch(proc.InitialCommand).Ch("timeouts").Incr()
			bolterror.NewBoltError(nil, "timeout", "API Call timeout, use id to fetch result", proc.InitialCommand, bolterror.Timeout).AddToPayload(proc.Payload)
			go engine.processCommands(proc, res, ch, q, true, true) //doesn't skip the current command object pushing to mq before waiting on the channel
			return

		case d = <-res:
			engine.LogDebug("cmd_complete", logrus.Fields{"id": proc.ID, "correlationId": d.CorrelationId, "command": proc.CurrentCommand.Name, "body": string(d.Body)}, "")

			//update command obj payload
			body, err := gabs.ParseJSON(d.Body)
			if err != nil {
				engine.Stats.Ch("commands").Ch(proc.CurrentCommand.Name).Ch("errors").Incr()
				bolterror.NewBoltError(err, proc.CurrentCommand.Name, "Command error: "+err.Error(), proc.InitialCommand, bolterror.Request).AddToPayload(proc.Payload)
				engine.completeProcess(proc, ch, q)
				return
			}
			proc.Payload = body

			//reset next command in prep
			proc.NextCommand = ""
			nexttmp := ""
			if proc.Payload.Path("nextCommand").Data() != nil {
				nexttmp = proc.Payload.Path("nextCommand").Data().(string)
			}

			//next command exists, so set it
			if nexttmp != "" {
				proc.NextCommand = nexttmp

				proc.Mutex.Lock()
				proc.Payload.SetP("", "nextCommand")
				proc.Mutex.Unlock()

				//check for 'circuit breaker' set in nextCommand, halt all further processing on this call
				if proc.NextCommand == HaltCallCommandName { // see constants.go - "HALT_CALL"
					engine.statCommandTime(proc)
					engine.statAPICallTime(proc)
					engine.Stats.Ch("performance").Ch("commands").Ch(proc.CurrentCommand.Name).Ch("halts").Incr()
					engine.completeProcess(proc, ch, q)
					engine.CacheCallResult(proc)
					engine.LogInfo("call_halt", logrus.Fields{"id": proc.ID, "last_command": proc.CurrentCommand.Name, "initial_input": proc.InitialInputString}, "")
					return
				}

				engine.LogDebug("cmd_found_next", logrus.Fields{"id": proc.ID, "next": proc.NextCommand}, "")

				//reached end of command list for this call, complete and return
			} else if proc.CurrentCommandIndex >= len(proc.APICall.Commands)-1 {
				engine.statCommandTime(proc)
				engine.statAPICallTime(proc)
				engine.completeProcess(proc, ch, q)
				engine.LogDebug("cmd_last_complete", logrus.Fields{"id": proc.ID}, proc.InitialCommand)
				if proc.CallType == commandprocess.CallTypeWork {
					engine.Requests.RemoveRequest(proc.ID)
				}
				engine.CacheCallResult(proc)
				return

				//reached end of a return after command, return but don't "complete" yet
			} else if proc.CurrentCommand.ReturnAfter {
				engine.statCommandTime(proc)
				engine.statAPICallTime(proc)
				proc.CurrentCommandIndex++
				proc.CurrentCommand = &proc.APICall.Commands[proc.CurrentCommandIndex]
				engine.LogDebug("cmd_return_after", logrus.Fields{"id": proc.ID, "command": proc.CurrentCommand.Name}, proc.InitialCommand)
				go engine.processCommands(proc, res, ch, q, false, true) //doesn't skip the current command object pushing to mq before waiting on the channel
				engine.CacheCallResult(proc)
				return

				//no next command, so setup & queue next main command
			} else if proc.NextCommand == "" {
				engine.statCommandTime(proc)
				proc.CurrentCommandIndex++
				proc.CurrentCommand = &proc.APICall.Commands[proc.CurrentCommandIndex]
				engine.LogDebug("cmd_next_main", logrus.Fields{"id": proc.ID, "command": proc.CurrentCommand.Name}, proc.InitialCommand)
			}
		}

		//make command request to mq
		if proc.NextCommand != "" {
			engine.LogDebug("cmd_queued_next", logrus.Fields{"id": proc.ID, "next": proc.NextCommand}, "")
			if engine.Config.Engine.TraceEnabled {
				proc.AddTraceEntry()
			}
			proc.Mutex.Lock()
			err = mqwrapper.PublishCommand(engine.mqConnection.Channel, proc.ID, engine.Config.Engine.Advanced.QueuePrefix, proc.NextCommand, proc.Payload, q.Name)
			proc.CommandTime = time.Now()
			if err != nil {
				engine.LogError("mq_error", logrus.Fields{"id": proc.ID, "command": proc.CurrentCommand.Name}, "Command failed to publish")
			}
			proc.NextCommand = ""
			proc.Mutex.Unlock()

		} else {
			engine.LogDebug("cmd_queued", logrus.Fields{"id": proc.ID, "next": proc.CurrentCommand.Name}, "")
			//setup command params
			proc.Mutex.Lock()
			proc.Payload.SetP(proc.CurrentCommand.ConfigParamsObj.Data(), "params")
			proc.Mutex.Unlock()

			if engine.Config.Engine.TraceEnabled {
				proc.AddTraceEntry()
			}
			proc.Mutex.Lock()
			err = mqwrapper.PublishCommand(engine.mqConnection.Channel, proc.ID, engine.Config.Engine.Advanced.QueuePrefix, proc.CurrentCommand.Name, proc.Payload, q.Name)
			proc.CommandTime = time.Now()
			if err != nil {
				engine.LogError("mq_error", logrus.Fields{"id": proc.ID, "command": proc.CurrentCommand.Name}, "Command failed to publish")
			}
			proc.Mutex.Unlock()
		}
	}

	//we're out of the loop so mark the request complete
	engine.completeProcess(proc, ch, q)
	engine.CacheCallResult(proc)
}

func (engine *Engine) completeProcess(proc *commandprocess.CommandProcess, ch *amqp.Channel, q *amqp.Queue) {
	if q != nil {
		//free the temp queue off mq server
		//err := engine.mqConnection.Channel.Cancel(q.Name, false)

		err := ch.Close()
		if err != nil {
			engine.LogWarn("queue_error", logrus.Fields{"id": proc.ID, "q": q.Name, "error": err}, "")
		}
	}
	proc.SetComplete()
}

func (engine *Engine) statCommandTime(proc *commandprocess.CommandProcess) {
	engine.Stats.Ch("performance").Ch("commands").Ch(proc.CurrentCommand.Name).Ch("avg_time").Avg(float32(time.Now().Sub(proc.CommandTime) / time.Millisecond))
}

func (engine *Engine) statAPICallTime(proc *commandprocess.CommandProcess) {
	engine.Stats.Ch("performance").Ch("calls").Ch(proc.InitialCommand).Ch("avg_time").Avg(float32(time.Now().Sub(proc.ReqTime) / time.Millisecond))
}
