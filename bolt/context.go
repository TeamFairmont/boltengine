// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/TeamFairmont/boltengine/throttling"
	"github.com/TeamFairmont/boltshared/config"
	"github.com/TeamFairmont/boltshared/security"
	"github.com/TeamFairmont/boltshared/utils"
	"github.com/sirupsen/logrus"
)

// Context containts the variables relevant to this http request (engine, logger, etc)
type Context struct {
	Engine      *Engine
	RequireAuth bool
}

// Handler contains the context and handler function for a bolt api http request
type Handler struct {
	Context   *Context
	H         func(*Context, http.ResponseWriter, *http.Request, string) error
	HMACGroup string
}

// ServeHTTP for bolt handles security (if applicable) and logging
// Output mime-type defaults to application/json
func (ah Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	ah.Context.Engine.Stats.Ch("general").Ch("requests_in").Incr()
	ah.Context.Engine.Stats.Ch("general").Ch("last_request_ip").Value(r.RemoteAddr)

	//handle CORS options request
	origin := r.Header.Get("Origin")
	if origin != "" {
		ah.Context.Engine.LogDebug("cors_origin", logrus.Fields{
			"method": r.Method, "url": r.URL.Path, "remoteaddr": r.RemoteAddr, "origin": origin,
		}, "CORS")

		ah.Context.Engine.Stats.Ch("general").Ch("cors_requests").Incr()

		if !utils.StringInSlice(origin, ah.Context.Engine.Config.Security.CorsDomains) {
			//check if corsDomains contains *, then check if origin contains what preceeds the *
			if !hasWild(origin, ah.Context.Engine.Config.Security.CorsDomains) {
				http.Error(w, http.StatusText(http.StatusPreconditionFailed), http.StatusPreconditionFailed)
				ah.Context.Engine.LogInfo("cors_error", logrus.Fields{
					"method": r.Method, "url": r.URL.Path, "remoteaddr": r.RemoteAddr, "origin": origin,
				}, "CORS")
				return
			} else { // * found in cores
				w.Header().Add("Access-Control-Allow-Origin", origin)
			}
		} else { //origin was found in slice
			w.Header().Add("Access-Control-Allow-Origin", origin)
		}

		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "Authorization, Bolt-No-Cache")
		w.Header().Add("Access-Control-Allow-Methods", "OPTIONS, GET, POST")
	}

	//some CORS use 'OPTIONS' requests, so write back nothing beyond the header
	if r.Method == "OPTIONS" {
		ah.Context.Engine.LogInfo("options_in", logrus.Fields{
			"method": r.Method, "url": r.URL.Path, "remoteaddr": r.RemoteAddr, "origin": origin,
		}, "CORS")
		return
	}

	// Attempt to decode all incoming messages

	// 'password' in the header is ignored and should be blank for hmac,
	// but it is used for simple authMode.
	groupname, password, ok := r.BasicAuth()
	ah.HMACGroup = groupname

	// Check if the handler is access controlled and if this group (if listed) has access.
	authed := true
	if ah.Context.RequireAuth {
		apiCall, err := ExtractCallName(r)
		if err != nil {
			apiCall = ""
		}
		authed = handlerAllowed(groupname, r.URL.Path, apiCall, ah.Context.Engine.Config.Security.HandlerAccess, ah)
	}

	// Throttle connections by groupname
	requestsPerSecond := throttle.GetThrottleForGroup(groupname, &ah.Context.Engine.Config.Security.Groups)
	if authed && requestsPerSecond > 0 {
		// This group has a limit on requests per second.  Check to see if the limit has been reached.
		if throttle.GroupLimitReached(ah.Context.Engine.Throttle, groupname, requestsPerSecond) {
			ah.Context.Engine.LogWarn("ServeHTTP-Throttled", logrus.Fields{"groupname": groupname, "requestsPerSecond": requestsPerSecond}, "Throttling- Returning error code 429 (Too Many Requests)")
			w.WriteHeader(429)
			return
		}
	}

	// Get the key for this group
	groupkey := ""
	var err error
	if authed && ok {
		ah.Context.Engine.LogInfo("http_in", logrus.Fields{
			"method": r.Method, "url": r.URL.Path, "remoteaddr": r.RemoteAddr, "groupname": groupname, "RequireAuth": ah.Context.RequireAuth,
		}, "Access")

		ah.Context.Engine.Stats.Ch("urls").Ch(r.URL.Path).Ch("hits").Incr()

		groupkey, err = security.GetKeyFromGroup(groupname, &ah.Context.Engine.Config.Security.Groups)

		if err != nil {
			authed = false
			ah.Context.Engine.LogWarn("security.GetKeyFromGroup", logrus.Fields{
				"method":     r.Method,
				"url":        r.URL.Path,
				"remoteaddr": r.RemoteAddr,
				"groupname":  groupname,
				"err":        err,
			}, "Failed to get key for group.  Does it exist in the config file?")
			ah.Context.Engine.Stats.Ch("general").Ch("group_not_found").Incr()
		}
	}

	if authed && !ok {
		ah.Context.Engine.LogInfo("http_in", logrus.Fields{
			"method": r.Method, "url": r.URL.Path, "remoteaddr": r.RemoteAddr, "RequireAuth": ah.Context.RequireAuth,
		}, "Access")

		authed = false
		if ah.Context.RequireAuth {
			ah.Context.Engine.LogWarn("r.BasicAuth()", logrus.Fields{
				"method":     r.Method,
				"url":        r.URL.Path,
				"remoteaddr": r.RemoteAddr,
				"groupname":  groupname,
				"ok":         ok,
			}, "Request header not ok")
			if ah.Context.Engine.Config.Engine.AuthModeValue == config.AuthModeSimple {
				w.Header().Set("WWW-Authenticate", "Basic realm=\"BoltEngine\"")
			}
		}
	} else if authed && ah.Context.Engine.Config.Engine.AuthModeValue == config.AuthModeSimple {
		//Do very basic auth
		if password != groupkey && password != "" {
			authed = false
			ah.Context.Engine.LogWarn("security.Simple", logrus.Fields{
				"method":     r.Method,
				"url":        r.URL.Path,
				"remoteaddr": r.RemoteAddr,
				"group":      groupname,
			}, "Key doesn't match group")
		} else {
			//if its POST, just pass through, but if its GET take the ?payload= variable and simulate a post
			if r.Method == "GET" {
				newRequest, err := http.NewRequest(
					"POST",     //	GET->POST,
					r.URL.Path, // URL- ignored,
					strings.NewReader(r.FormValue("payload")), // GET payload
				)
				if err != nil {
					authed = false
					ah.Context.Engine.LogWarn("http.NewRequest", logrus.Fields{
						"method":     r.Method,
						"url":        r.URL.Path,
						"remoteaddr": r.RemoteAddr,
						"err":        err,
					}, "Failed to generate the body payload")
				} else {
					r.Method = "POST"
					r.Body = newRequest.Body
				}
			}
		}
	} else if authed {
		// Header contains BasicAuth for groupname only
		// Read the body and decode it, using the groupname in the header to get the HMAC key.
		rBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			authed = false
			ah.Context.Engine.LogWarn("ioutil.ReadAll(r.Body)", logrus.Fields{
				"method":     r.Method,
				"url":        r.URL.Path,
				"remoteaddr": r.RemoteAddr,
				"err":        err,
			}, "Unable to read request body")
		}

		// Decode the request
		rBodyDecoded, err := security.DecodeHMAC(
			groupkey,
			rBody,
			ah.Context.Engine.Config.Security.VerifyTimeout,
		)

		if err != nil {
			authed = false
			if ah.Context.RequireAuth {
				ah.Context.Engine.LogWarn("security.DecodeHMAC", logrus.Fields{
					"method":       r.Method,
					"url":          r.URL.Path,
					"remoteaddr":   r.RemoteAddr,
					"rBodyDecoded": "",
					"err":          err,
				}, "Failed to decode the request")
			}
			ah.Context.Engine.Stats.Ch("general").Ch(groupname).Ch("decode_failures").Incr()
		} else {

			// Replace the original request's body contents by
			// generating a new request containing the decoded body,
			// then replacing the original request body with the new request's body
			newRequest, err := http.NewRequest(
				r.Method,                        //	POST- ignored,
				r.URL.Path,                      // URL- ignored,
				strings.NewReader(rBodyDecoded)) // Decoded HMAC body
			if err != nil {
				authed = false
				ah.Context.Engine.LogWarn("http.NewRequest", logrus.Fields{
					"method":     r.Method,
					"url":        r.URL.Path,
					"remoteaddr": r.RemoteAddr,
					"err":        err,
				}, "Failed to generate the decoded request")
			} else {
				r.Body = newRequest.Body
			}

		}
	} // /BasicAuth ok

	// If auth is required, only handle messages that have been decoded (authed==true)
	// OR auth isn't required, so handle messages regardless of if they're un-encoded or decoded.
	if (ah.Context.RequireAuth && authed) || !ah.Context.RequireAuth {
		ah.Context.Engine.Stats.Ch("general").Ch("authed_requests").Incr()
		ah.Context.Engine.Stats.Ch("security").Ch(groupname).Ch("authed_requests").Incr()

		// pass context as a parameter to our handler
		w.Header().Set("Content-Type", "application/json")
		err := ah.H(ah.Context, w, r, ah.HMACGroup)
		if err != nil {
			ah.Context.Engine.LogError("http_in", logrus.Fields{
				"method":      r.Method,
				"url":         r.URL.Path,
				"remoteaddr":  r.RemoteAddr,
				"error":       err,
				"RequireAuth": ah.Context.RequireAuth,
			}, "")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

	} else {
		// Auth is required and this request wasn't decoded (authed==false)
		ah.Context.Engine.Stats.Ch("general").Ch("auth_failed_count").Incr()
		ah.Context.Engine.Stats.Ch("general").Ch("last_auth_fail_ip").Value(r.RemoteAddr)
		ah.Context.Engine.Stats.Ch("security").Ch(groupname).Ch("auth_failed_count").Incr()
		ah.Context.Engine.LogWarn("auth_fail", logrus.Fields{"remoteaddr": r.RemoteAddr}, "Key auth failed")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}

}

// handlerAllowed is called when auth is required.  It checks that the submitted groupname is explicitely denied access.
// If handler access is limited by an allow list, it checks that the submitted groupname is on the list.
func handlerAllowed(groupname, url, apiCall string, handlerAccess []config.HandlerAccess, ah Handler) bool {

	// Loop through the list of access controlled handlers
	for handlerIndex := range handlerAccess {

		// Check to see if the requested url contains a restricted handler name
		if (handlerAccess[handlerIndex].APICall != "" && handlerAccess[handlerIndex].APICall == apiCall) ||
			(handlerAccess[handlerIndex].HandlerURL != "" && strings.HasSuffix(url, handlerAccess[handlerIndex].HandlerURL)) {

			//// Check to see if this group is denied
			if len(handlerAccess[handlerIndex].DenyGroups) > 0 && groupname == "" {
				// A deny list exists for this handler and no groupname was listed.  Deny access.
				ah.Context.Engine.LogWarn("handler_access_control_missing_groupname", logrus.Fields{
					"url":        url,
					"handlerURL": handlerAccess[handlerIndex].HandlerURL,
					"apiCall":    handlerAccess[handlerIndex].APICall,
					"denyGroups": handlerAccess[handlerIndex].DenyGroups,
				}, "Deny list exists, no groupname given.")
				return false
			}
			// Loop through the list of denied groups, checking for the groupname attempting access
			for groupIndex := range handlerAccess[handlerIndex].DenyGroups {
				if groupname == handlerAccess[handlerIndex].DenyGroups[groupIndex] {
					// This groupname is on the deny list
					ah.Context.Engine.LogWarn("handler_access_control_denied_groupname", logrus.Fields{
						"groupname":   groupname,
						"denyGroups":  handlerAccess[handlerIndex].DenyGroups,
						"request_url": url,
					}, "Handler Access Denied For Group")
					return false
				}
			}

			//// Check to see if there's an allow list.  If so, check that this group is allowed.
			if len(handlerAccess[handlerIndex].AllowGroups) > 0 {
				// Loop through the allowed groups, return true if the groupname is on it.  Otherwise return false.
				for groupIndex := range handlerAccess[handlerIndex].AllowGroups {
					if groupname == handlerAccess[handlerIndex].AllowGroups[groupIndex] {
						return true
					}
				}
				// There's a list of groups to allow and this group isn't on it.
				ah.Context.Engine.LogWarn("handler_access_control_not_allowed", logrus.Fields{
					"allowGroups": handlerAccess[handlerIndex].AllowGroups,
					"request_url": url,
					"groupname":   groupname,
				}, "Handler Access Not Allowed")
				// Your name's not down, you're not coming in
				return false
			}

		}
	}

	// Group not explicitly allowed or denied.  Allow access by default.
	return true
}

// hasWild checks the cors domains for *
func hasWild(origin string, cors []string) bool {
	for _, domain := range cors { //range over list of corsDomains
		if strings.ContainsAny(domain, "*") { //check the strings for *
			splitDomain := strings.Split(domain, "*")     //split the strings on *
			if strings.Contains(origin, splitDomain[0]) { //if the origin contains the splitDomain
				return true //wildcard found
			}
		}
	}
	return false //wildcard not found
}
