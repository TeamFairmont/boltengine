// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"errors"
	"time"

	"github.com/TeamFairmont/boltshared/config"
	"github.com/TeamFairmont/boltshared/mqwrapper"
	"github.com/TeamFairmont/boltshared/utils"
	"github.com/TeamFairmont/gabs"
	"github.com/sirupsen/logrus"
)

var count = 0

func (engine *Engine) workerStub() {
	engine.LogWarn("worker_stub", nil, "stubMode true, running a worker stub for all config'ed commands!")

	mq, err := mqwrapper.ConnectMQ(engine.Config.Engine.MQUrl)
	if err != nil {
		engine.LogWarn("worker_stub", logrus.Fields{"err": err}, "Worker stub failed to connect to the MQ")
	}

	err = mq.Channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		engine.LogWarn("worker_stub", logrus.Fields{"err": err}, "Worker stub QoS couldn't be set")
	}

	//make an all commands map to de-dupe if same command in multiple calls
	allcommands := make(map[string]*config.CommandInfo)
	for _, call := range engine.Config.APICalls {
		for j := range call.Commands {
			meta, ok := engine.Config.CommandMetas[call.Commands[j].Name]
			if !ok || !meta.NoStub { //skip those with no-stub
				allcommands[call.Commands[j].Name] = &call.Commands[j]
			}
		}
	}

	//TODO error if required params missing from stubs
	//spin up queues and goroutines for each command
	count = 0
	for k, cmd := range allcommands {
		q, res, err := mqwrapper.CreateConsumeNamedQueue(engine.Config.Engine.Advanced.QueuePrefix+k, mq.Channel)
		if err != nil {
			engine.LogWarn("worker_stub", logrus.Fields{"command": k, "err": err}, "Worker stub failed to register queue")
		} else {
			//start goroutine on res chan
			name := k
			command := cmd
			meta, metaok := engine.Config.CommandMetas[name]
			go func(k string) {
				currentIteration := utils.GetBoltIteration()
				//waits here untill a worker stub is queried, then infinite loop
				for d := range res {
					keepAlive, err := utils.CheckBoltIteration(currentIteration)
					if err != nil {
						engine.LogError("worker_stub", logrus.Fields{"Error": err}, "Error with CheckBoltIteration(), in workerStub()")
					}
					engine.LogDebug("worker_stub", logrus.Fields{"command": q.Name, "id": d.CorrelationId, "payload": string(d.Body)}, "Command received")

					payload, err := gabs.ParseJSON(d.Body)
					if err != nil {
						engine.LogError("worker_stub", logrus.Fields{"command": q.Name, "id": d.CorrelationId, "payload": string(d.Body)}, "Payload malformed, not valid JSON")
					}
					// Get the requiredParams from the desired command name "k" from the config
					reqParams := engine.Config.CommandMetas[k].RequiredParams
					// Check for requiredParams
					for reqParamKey := range reqParams {
						if !payload.Path("initial_input").Exists(reqParamKey) {
							// if the required parameters are not in initial_input
							children, err := payload.Path("trace").Path("command").Children()
							if err != nil {
								engine.LogError("RequiredParams", logrus.Fields{"ev": err}, "Error with reqiuredParams gabs children")
							}
							for _, child := range children {
								engine.LogError("RequiredParams", logrus.Fields{"ev": errors.New("Missing Required Parameters")}, "Error Command "+child.Data().(string)+" Required Parameters Missing")
							}
						}
					}

					//stub data values
					if metaok && meta.StubData != nil {
						stub, err := gabs.ParseJSON(meta.StubData)
						if err != nil {
							engine.LogError("worker_stub", logrus.Fields{"command": command, "err": err, "stubData": meta.StubData}, "Worker stub couldn't parse stubData meta")
						}
						stubchildren, err := stub.ChildrenMap()
						if err != nil {
							engine.LogError("worker_stub", logrus.Fields{"command": command, "err": err}, "Worker stub couldn't get stubData children map")
						}
						for stubk, stubv := range stubchildren {
							_, err := payload.Set(stubv.Data(), "data", stubk)
							if err != nil {
								engine.LogError("worker_stub", logrus.Fields{"command": command, "err": err, "value": stubv, "key": stubk}, "Worker stub couldn't add stubData values to payload data")
							}
						}
					}

					//stub return values
					if metaok && meta.StubReturn != nil {
						stub, err := gabs.ParseJSON(meta.StubReturn)
						if err != nil {
							engine.LogError("worker_stub", logrus.Fields{"command": command, "err": err, "stubReturn": meta.StubReturn}, "Worker stub couldn't parse stubReturn meta")
						}
						stubchildren, err := stub.ChildrenMap()
						if err != nil {
							engine.LogError("worker_stub", logrus.Fields{"command": command, "err": err}, "Worker stub couldn't get stubReturn children map")
						}
						for stubk, stubv := range stubchildren {
							_, err := payload.Set(stubv.Data(), "return_value", stubk)
							if err != nil {
								engine.LogError("worker_stub", logrus.Fields{"command": command, "err": err, "value": stubv, "key": stubk}, "Worker stub couldn't add stubData values to payload return_value")
							}
						}
					}

					//append to "stub" call path string
					stub := utils.NilString(payload.Path("debug.stub").Data(), "")
					stub += name + "|"

					_, err = payload.SetP(stub, "debug.stub")
					if err != nil {
						engine.LogError("worker_stub", logrus.Fields{"command": command, "err": err}, "Worker stub failed to set debug value")
					}

					//simulate compute time
					if metaok && meta.StubDelayMs != 0 {
						time.Sleep(time.Duration(meta.StubDelayMs) * time.Millisecond)
					} else {
						time.Sleep(time.Duration(engine.Config.Engine.Advanced.StubDelayMs) * time.Millisecond)
					}

					//send reply back
					err = mqwrapper.PublishCommand(mq.Channel, d.CorrelationId, "", d.ReplyTo, payload, "")
					if err != nil {
						engine.LogError("worker_stub", logrus.Fields{"command": command, "err": err}, "Worker stub failed to publish command result")
					}
					//ack to queue that this message is done
					d.Ack(false)
					engine.LogInfo("worker_stub", logrus.Fields{"command": q.Name, "id": d.CorrelationId}, "Command completed")

					//happens after reboot but only after a api call
					if !keepAlive {
						return
					}
				}
			}(k)
			engine.LogInfo("worker_stub", logrus.Fields{"command": name}, "Worker stub registered for command")
		}
	}
	engine.LogInfo("worker_stub", nil, "Worker Stub waiting for commands...")
}
