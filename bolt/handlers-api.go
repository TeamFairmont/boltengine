// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/TeamFairmont/boltengine/bolterror"
	"github.com/TeamFairmont/boltengine/commandprocess"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func apicallHandlers(engine *Engine) {

	//insert a command into the mq with initial payload, returns no id, disregards result immediately
	engine.Mux.Handle("/work/", Handler{Context: engine.ContextAuth,
		H: func(ctx *Context, w http.ResponseWriter, r *http.Request, HMACGroup string) error {
			engine.HandleCall(commandprocess.CallTypeWork, w, r, HMACGroup)
			return nil
		}})

	//insert a command into the mq with initial payload. don't wait for result, just return initial req_id immediately
	engine.Mux.Handle("/task/", Handler{Context: engine.ContextAuth,
		H: func(ctx *Context, w http.ResponseWriter, r *http.Request, HMACGroup string) error {
			engine.HandleCall(commandprocess.CallTypeTask, w, r, HMACGroup)
			return nil
		}})

	//insert a command into the mq with initial payload, wait for and return result
	engine.Mux.Handle("/request/", Handler{Context: engine.ContextAuth,
		H: func(ctx *Context, w http.ResponseWriter, r *http.Request, HMACGroup string) error {
			engine.HandleCall(commandprocess.CallTypeRequest, w, r, HMACGroup)
			return nil
		}})

	//***************************************
	//for advanced routes with variables, etc
	//***************************************
	hand := mux.NewRouter()

	//returns current state of an id request, clears from the request list if complete, clears from list no matter what if 'fetch'ing
	//also accepts 'status' in the peekOrFetch param to get the engine status info of the request
	hand.Handle("/retr/{peekOrFetch}/{id}", Handler{Context: engine.ContextAuth,
		H: func(ctx *Context, w http.ResponseWriter, r *http.Request, HMACGroup string) error {
			w.Header().Set("Content-Type", "application/json")

			vars := mux.Vars(r)
			req := ctx.Engine.Requests.GetRequest(vars["id"])
			if req == nil {
				ctx.Engine.OutputError(w, bolterror.NewBoltError(nil, "retr", "Invalid request ID", vars["id"], bolterror.Request))
			} else {
				req.UpdatePeekTime()

				req.Payload.SetP(req.Complete, "complete")

				if vars["peekOrFetch"] == "fetch" {
					ctx.Engine.OutputRequest(w, req, req.APICall.FilterKeys)
					engine.LogInfo("call_out", logrus.Fields{"id": req.ID, "vars": vars, "command": req.InitialCommand}, "")
					ctx.Engine.Requests.RemoveRequest(vars["id"])
				} else if vars["peekOrFetch"] == "status" {
					engine.LogInfo("status_call", logrus.Fields{"vars": vars, "command": req.InitialCommand}, "")
					reqjson, _ := json.MarshalIndent(req, "", "\t")
					fmt.Fprint(w, string(reqjson))
				} else {
					ctx.Engine.OutputRequest(w, req, req.APICall.FilterKeys)
					engine.LogInfo("peek_call", logrus.Fields{"vars": vars, "command": req.InitialCommand}, "")
					if req.Complete {
						ctx.Engine.Requests.RemoveRequest(vars["id"])
					}
				}
			}

			return nil
		}})

	engine.Mux.Handle("/retr/", hand)
}
