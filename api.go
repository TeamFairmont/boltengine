// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/TeamFairmont/boltengine/bolt"
	"github.com/TeamFairmont/boltshared/config"
	"github.com/TeamFairmont/boltshared/utils"
	"github.com/sirupsen/logrus"
)

var (
	cfgdir     = "/etc/bolt" // Dir containing the customized config.json
	cfgpath    bytes.Buffer  // Full path to config.json. Typically /etc/bolt/config.json
	engine     *bolt.Engine
	loop       = true            // controlls main loop
	rebootChan = make(chan bool) // controlls main loop reboot
	ch         = make(chan bool) // signals the restart proccess
	first      = true
)

// APIReboot starts the reboot listener
func APIReboot(ch, rebootChan chan bool) {
	bolt.EngineReboot(ch, rebootChan)
}

// startEngine is a large portion of what was in main of api.go
func startEngine(ch chan bool, rebootChan chan bool) {

	utils.IncrementBoltIteration()
	go func() {
		utils.InitDoneChan()
		engine = &bolt.Engine{}
		engine.Log = logrus.StandardLogger()
		// gets the stoppable listener and waits for the reboot signal
		go func(engine *bolt.Engine) {
			lch := bolt.GetListenChan()
			if <-ch { // when ch is recieved begin restarting
				lch.Stop() // stop the stoppable listener
			}
		}(engine)

		// Add a slash to the end of cfgdir if needed.
		if !strings.HasSuffix(cfgdir, "/") {
			cfgdir += "/"
		}
		// cfgpath has to be cleard for each reboot
		cfgpath = *bytes.NewBufferString("")
		cfgpath.WriteString(cfgdir)
		cfgpath.WriteString("config.json")
		engine.ConfigPath = cfgpath.String() // Typically this is: "/etc/bolt/config.json"

		cfg, err := config.BuildConfig(cfgdir, engine.ConfigPath)
		if err != nil {
			engine.LogFatal("init", logrus.Fields{"error": err}, "Error building config")
		}

		engine.PostConfig(cfg)
		bolt.BuiltinHandlers(engine)

		strcfg, _ := json.Marshal(engine.Config)
		engine.LogDebug("init_info", logrus.Fields{"config": string(strcfg)}, "Config Loaded")

		if engine.ListenAndServe() != nil {
			engine.LogFatal("critical", nil, "Error starting server, check config and ports")
		}
	}()
}

// CheckConfig checks the config for changes, ignores first change on startup
func CheckConfig(ch chan bool, rebootChan chan bool) {
	count := 1
	start := time.Now()
	for true {
		// find config files last modified date
		fi, err := os.Stat(cfgdir + "/config.json")
		if err != nil {
			engine.LogError("init", logrus.Fields{"error": err}, "Error reading config file's modified date")
			return
		} // skip the first difference, on startup
		if count == 1 {
			count--
			start = fi.ModTime() // set start to the files last modified time
		}
		// check if the file modified time is different then the starting time
		if fi.ModTime() != start {
			start = fi.ModTime()
			bolt.StartEngineReboot()
		} // sleep before re checking
		time.Sleep(2 * time.Second)
	}
}
func main() {
	utils.InitBoltIteration()
	// Start the reboot listener
	go APIReboot(ch, rebootChan)
	// checks for changes to config.json
	go CheckConfig(ch, rebootChan)

	// main loop
	for loop {
		// setup and start engine
		startEngine(ch, rebootChan) // is a go routine
		// got the signal time to restart
		loop = <-rebootChan
	}
}
