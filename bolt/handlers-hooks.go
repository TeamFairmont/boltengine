// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
)

// serveSingle allows individual files to be served.  Useful for css, js, or html
func serveSingle(engine *Engine, pattern string, filename string) {
	// Check that the static file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		dir, direrr := os.Getwd()
		if direrr != nil {
			engine.LogInfo("init", logrus.Fields{"err": direrr}, "Error obtaining working dir")
		}
		engine.LogInfo("init", logrus.Fields{"err": err, "work_dir": dir}, "Missing static file")
	}
	// Create the single file handler
	engine.Mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filename)
	})
}

// BuiltinHandlers registers the core bolt engine calls
func BuiltinHandlers(eng *Engine) {
	eng.ContextNoAuth = &Context{Engine: eng, RequireAuth: false}
	eng.ContextAuth = &Context{Engine: eng, RequireAuth: true}

	//static handlers
	//js
	serveSingle(eng, "/static/js/jquery.js", "./html_static/static/js/jquery.js")
	serveSingle(eng, "/static/js/crypto-js/aes.js", "./html_static/static/js/CryptoJSv312/rollups/aes.js")
	serveSingle(eng, "/static/js/crypto-js/enc-base64-min.js", "./html_static/static/js/CryptoJSv312/components/enc-base64-min.js")
	serveSingle(eng, "/static/js/crypto-js/hmac-sha512.js", "./html_static/static/js/CryptoJSv312/rollups/hmac-sha512.js")
	serveSingle(eng, "/static/js/base64.js", "./html_static/static/js/base64.js")
	//css
	serveSingle(eng, "/static/css/foundation.min.css", "./html_static/static/css/foundation.min.css")
	serveSingle(eng, "/static/css/custom.css", "./html_static/static/css/custom.css")

	//core handlers

	//display the commands and their required paramets
	eng.Mux.Handle("/docs", Handler{Context: eng.ContextNoAuth, H: coreHandleDocs})

	//no security 'test' for client devs to check connectivity
	eng.Mux.Handle("/test", Handler{Context: eng.ContextNoAuth, H: coreHandleTest})

	//outputs its input as 'echo'
	eng.Mux.Handle("/echo/", Handler{Context: eng.ContextAuth, H: coreHandleEcho})

	//grabs server 'time'
	eng.Mux.Handle("/time", Handler{Context: eng.ContextAuth, H: coreHandleTime})

	//outputs debug form for api call
	eng.Mux.Handle("/form/", Handler{Context: eng.ContextNoAuth, H: coreHandleDebugForm})

	//outputs recent debug log
	eng.Mux.Handle("/debug-log", Handler{Context: eng.ContextAuth, H: coreHandleDebugLog})

	//outputs this engines general stats
	eng.Mux.Handle("/stats", Handler{Context: eng.ContextAuth, H: coreHandleStats})

	//lists this engines pending requests
	eng.Mux.Handle("/pending", Handler{Context: eng.ContextAuth, H: coreHandlePending})

	//outputs this engines effective config
	eng.Mux.Handle("/get-config", Handler{Context: eng.ContextAuth, H: coreHandleGetConfig})

	//save custom bolt configs and reload the config
	eng.Mux.Handle("/save-config", Handler{Context: eng.ContextAuth, H: coreHandleSaveConfig})

	// handler for engine reboot
	eng.Mux.Handle("/engine-reboot", Handler{Context: eng.ContextAuth, H: coreHandleReboot})

	//handlers for custom api calls
	apicallHandlers(eng)
}
