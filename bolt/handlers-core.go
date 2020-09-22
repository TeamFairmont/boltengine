// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/user"
	"strings"
	"time"

	"github.com/TeamFairmont/boltengine/bolterror"
	"github.com/TeamFairmont/boltshared/mqwrapper"
	"github.com/TeamFairmont/boltshared/utils"
	"github.com/TeamFairmont/gabs"
	"github.com/sirupsen/logrus"
)

var (
	startRebootChan = make(chan bool)
)

// StartEngineReboot starts the reboot proccess called from api.CheckConfig or coreHandleReboot
func StartEngineReboot() {
	startRebootChan <- true
}

// EngineReboot starts the reboot process started in api.go,
func EngineReboot(ch, rebootChan chan bool) {
	for true {
		<-startRebootChan        // wait for signal from startRebootChan called from coreHandleReboot or CheckConfig in api.go
		utils.CloseDoneChan()    // Closes doneChan, that will close several go routines, engine.go: 406 in workerErrorQueue, engine.go: 480 in expireResults(), requestmanager: 68 in NewRequestManager()
		ch <- true               // send to ch in api.go startEngine() to close the stoppable listenerD
		mqwrapper.CloseRes()     // closes out of a goroutine in engine.go workerErrorQueue(), by closing res, by closing the connection in mqwrapper.go CreateConsumeNamedQueue
		<-utils.GetResDoneChan() // wait for the tcp port to finish closing
		rebootChan <- true       // send to rebootChan in api.go to restart main loop
	}
}

// coreHandleReboot sends a signal to start reboot
func coreHandleReboot(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	StartEngineReboot()
	return nil
}

func coreHandleTest(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	fmt.Fprintf(w, "{\"test\": %d}", 1)
	return nil
}

func coreHandleEcho(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	fmt.Fprintf(w, "{\"echo\": %q}", r.URL.String())
	return nil
}

func coreHandleTime(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	fmt.Fprintf(w, "{\"time\": %d}", time.Now().UnixNano())
	return nil
}

// coreHandleDebugForm handles /form/
func coreHandleDebugForm(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	if strings.ToUpper(r.Method) == "GET" && ctx.Engine.Config.Engine.Advanced.DebugFormEnabled {
		ctx.Engine.OutputDebugForm(w, r)
		ctx.Engine.LogDebug("debugForm", logrus.Fields{
			"url": r.URL.String(),
		}, "Debug form served")
	} else {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
	return nil
}

func coreHandleDebugLog(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	w.Header().Set("Content-Type", "text/plain")
	fc, _ := ioutil.ReadFile("/tmp/fairmont.debug.log")
	fmt.Fprint(w, string(fc[:]))
	return nil
}

func coreHandleStats(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	stats, _ := ctx.Engine.Stats.JSON()
	fmt.Fprint(w, stats)
	return nil
}

// coreHandleGetConfig should restrict access using config.json > security > handlerAccess > handler":"/get-config", "allowGroups":["allowed_groupname_here"]
func coreHandleGetConfig(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	c, err := ctx.Engine.Config.JSON()
	if err != nil {
		ctx.Engine.OutputError(w, bolterror.NewBoltError(err, "get-config", "Invalid config, couldn't convert to JSON", "", bolterror.Internal))
		return err
	}
	fmt.Fprint(w, c)
	return nil
}

// coreHandleSaveConfig should restrict access using config.json > security > handlerAccess > handler":"/get-config", "allowGroups":["allowed_groupname_here"]
func coreHandleSaveConfig(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	// Error messages sent back to the client are intentionally vague when it comes to security.
	// They only need to know of a security error, but shouldn't be given hints about whether it was due to a bad shared key, or expired timestamp.
	var securityError = false

	// Get the request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		securityError = true
		ctx.Engine.LogError("ioutil.ReadAll(r.Body)", logrus.Fields{
			"method":     r.Method,
			"url":        r.URL.Path,
			"remoteaddr": r.RemoteAddr,
			"err":        err,
		}, "Error reading request body of new config")
	}

	// Save the config out to a file
	if !securityError {
		jsonByte := []byte(body)
		err = ioutil.WriteFile(ConfigPath, jsonByte, 0600)
		if err != nil {
			securityError = true
			currentUser, err := user.Current()
			if err != nil {
				ctx.Engine.LogWarn("ioutil.WriteFile", logrus.Fields{
					"method":     r.Method,
					"url":        r.URL.Path,
					"remoteaddr": r.RemoteAddr,
					"err":        err,
				}, "Error obtaining username")
			}
			ctx.Engine.LogError("coreHandleSaveConfig", logrus.Fields{
				"method":               r.Method,
				"url":                  r.URL.Path,
				"remoteaddr":           r.RemoteAddr,
				"err":                  err,
				"currentUser.Username": currentUser.Username,
				"customCfgpath":        ConfigPath,
			}, "Error writing to config file.  Does the currentUser.Username have permission to edit customCfgpath?")

			permissionFix := []string{"Fix file permissions (may need root/sudo to perform): mkdir /etc/bolt; cp etc/bolt/config.json /etc/bolt/; chown ", currentUser.Username, " ", ConfigPath, "; chmod 0600 ", ConfigPath}
			ctx.Engine.LogError("coreHandleSaveConfig", logrus.Fields{
				"method":     r.Method,
				"url":        r.URL.Path,
				"remoteaddr": r.RemoteAddr,
			}, strings.Join(permissionFix, ""))
		}
	}

	if securityError {
		// Respond to the client with a status of Unauthorized
		ctx.Engine.LogError("coreHandleSaveConfig", logrus.Fields{
			"method":     r.Method,
			"url":        r.URL.Path,
			"remoteaddr": r.RemoteAddr,
		}, "Response Payload error")
		fmt.Fprint(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return nil
	}

	ctx.Engine.LogInfo("coreHandleSaveConfig", logrus.Fields{
		"method":                r.Method,
		"url":                   r.URL.Path,
		"remoteaddr":            r.RemoteAddr,
		"ctx.Engine.ConfigPath": ctx.Engine.ConfigPath,
	}, "Saved new config file")

	fmt.Fprint(w, http.StatusText(http.StatusAccepted), http.StatusAccepted)

	// Flush the response
	if f, ok := w.(http.Flusher); ok {
		ctx.Engine.LogInfo("coreHandleSaveConfig", logrus.Fields{
			"method":     r.Method,
			"url":        r.URL.Path,
			"remoteaddr": r.RemoteAddr,
		}, "Flushing")
		f.Flush()
	} else {
		ctx.Engine.LogInfo("coreHandleSaveConfig", logrus.Fields{
			"method":                r.Method,
			"url":                   r.URL.Path,
			"remoteaddr":            r.RemoteAddr,
			"ctx.Engine.ConfigPath": ctx.Engine.ConfigPath,
		}, "Flush isn't supported")
	}

	// Restart   If running as a systemd service as described in the README.md, the api will automatically restart if it's not running.
	// When the api restarts, it will load the new config file.
	ctx.Engine.LogWarn("os.Exit(0)", logrus.Fields{
		"method":     r.Method,
		"url":        r.URL.Path,
		"remoteaddr": r.RemoteAddr,
	}, "Exiting   Service will restart in approximately 30 seconds if the Bolt API is run as a systemd service.  See README.md for more information.")

	ctx.Engine.Shutdown()
	return nil // Unreachable

}

func coreHandleDocs(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {

	// Remove double quotes and braces from a string.  Also add space after commmas.
	trimString := func(theString string) string {
		theString = strings.Trim(theString, "{")
		theString = strings.Trim(theString, "}")
		theString = strings.Trim(theString, "[")
		theString = strings.Trim(theString, "]")
		theString = strings.Replace(theString, "\"", "", -1)  // remove double quote
		theString = strings.Replace(theString, ",", ", ", -1) // add space after comma
		return theString
	}

	// Add html row with call info
	addRow := func(call *gabs.Container, friendlyName string, callPath string) string {
		var buffer bytes.Buffer
		buffer.WriteString("<div class='row collapse prefix-radius'><div class='small-3 columns'><span class='prefix'>")
		buffer.WriteString(friendlyName)
		buffer.WriteString("</span></div><div class='small-9 columns'>")
		buffer.WriteString("<input type='text' disabled='' value='")
		trimReqs := call.Path(callPath).String()
		buffer.WriteString(trimString(trimReqs))
		buffer.WriteString("'>")
		buffer.WriteString("</div></div>")
		return buffer.String()
	}

	w.Header().Set("Content-Type", "text/html")

	var buffer bytes.Buffer

	if !ctx.Engine.Config.Engine.DocsEnabled {
		buffer.WriteString("Docs disabled")
	} else {
		config, _ := ctx.Engine.Config.JSON()
		configParsed, _ := gabs.ParseJSON([]byte(config))

		buffer.WriteString("<link rel='stylesheet' href='/static/css/foundation.min.css'>")
		buffer.WriteString("<link rel='stylesheet' href='/static/css/custom.css'>")
		buffer.WriteString("<head>")
		//javascrip function definition for more / less description
		buffer.WriteString("<script>")
		buffer.WriteString(moreJS)
		buffer.WriteString("</script>")
		buffer.WriteString("</head>")
		buffer.WriteString("<body class='docs'><div class='row'><div class='large-12 columns'>")
		buffer.WriteString("<h1>Bolt | Command List</h1>")

		// API Calls
		buffer.WriteString("<h2>API Calls</h2>")
		apiCalls, _ := configParsed.S("apiCalls").ChildrenMap()
		for callKey, call := range apiCalls {
			buffer.WriteString("<h3><a href='/form/")
			buffer.WriteString(callKey)
			buffer.WriteString("'>")
			buffer.WriteString(callKey)
			buffer.WriteString("</a></h3>")
			//html for more / less description
			buffer.WriteString(moreHTML(callKey, call.Path("shortDescription").Data().(string), call.Path("longDescription").Data().(string)))
			buffer.WriteString(runToggleDescription(callKey, call.Path("shortDescription").Data().(string), call.Path("longDescription").Data().(string)))
			buffer.WriteString("<div class='row commandinfo'><div class='large-12 columns'>")
			buffer.WriteString(addRow(call, "Required Params", "requiredParams"))
			buffer.WriteString(addRow(call, "Result Timeout (ms)", "resultTimeoutMs"))
			buffer.WriteString(addRow(call, "Cache Enabled", "cache"))
			buffer.WriteString(addRow(call, "Commands", "commands.name"))
			buffer.WriteString("</div></div>")
		}
		buffer.WriteString("<hr>")

		// Command Meta
		buffer.WriteString("<h2>Command Meta</h2>")
		commandMeta, _ := configParsed.S("commandMeta").ChildrenMap()
		for callKey, call := range commandMeta {
			buffer.WriteString("<h3>")
			buffer.WriteString(callKey)
			buffer.WriteString("</h3>")
			//html for more / less description
			buffer.WriteString(moreHTML(callKey, call.Path("shortDescription").Data().(string), call.Path("longDescription").Data().(string)))
			buffer.WriteString(runToggleDescription(callKey, call.Path("shortDescription").Data().(string), call.Path("longDescription").Data().(string)))
			buffer.WriteString("<div class='row commandinfo'><div class='large-12 columns'>")
			buffer.WriteString(addRow(call, "Required Params", "requiredParams"))
			buffer.WriteString("</div></div>")
		}
		buffer.WriteString("</div></div></body>")
	}

	fmt.Fprint(w, buffer.String())
	return nil
}

// TODO make a coreHandleRestart

func coreHandlePending(ctx *Context, w http.ResponseWriter, r *http.Request, group string) error {
	reqs, err := ctx.Engine.Requests.StatusJSON()
	fmt.Fprint(w, reqs)
	return err
}

//******* /form helpers **********

//JS function for more / less description for apicalls and command metas
var moreJS = `
//initialize map of statuses
var statusMap = new Map();
function toggleDescription(longDescription, shortDescription, apicall){
    if (!statusMap.has(apicall)){
        statusMap.set(apicall, "less");
        document.getElementById(apicall+"DescriptionArea").innerHTML = shortDescription;
        if(longDescription.length > 0){
            document.getElementById(apicall+"ToggleButton").innerText = "See More";
            }
        statusMap.set(apicall, "less");
    } else if (statusMap.get(apicall) == "less") {
        document.getElementById(apicall+"DescriptionArea").innerHTML=longDescription;
        document.getElementById(apicall+"ToggleButton").innerText = "See Less";
        statusMap.set(apicall, "more");
    } else if (statusMap.get(apicall) == "more" ) {
        document.getElementById(apicall+"DescriptionArea").innerHTML = shortDescription;
        document.getElementById(apicall+"ToggleButton").innerText = "See More";
        statusMap.set(apicall, "less");
    }
    
};
`

//runToggleDescription runs the javascrip function
func runToggleDescription(apicall, shortDescription, longDescription string) string {
	return fmt.Sprintf("<script>toggleDescription('%s','%s','%s');</script>", longDescription, shortDescription, apicall)
}

// moreHTML parses the variables for more / less button
func moreHTML(callKey, shortDescription, longDescription string) string {
	var HTML = fmt.Sprintf(`
            <div>
                <p style="display:inline" id="%sDescriptionArea">%s </p> 
                <a id="%sToggleButton" onclick="(toggleDescription('%s','%s','%s'))" href="javascript:void(0);"></a>
                </div>
            `, callKey, shortDescription, callKey, longDescription, shortDescription, callKey)
	return HTML
}
