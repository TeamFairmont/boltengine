// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/TeamFairmont/boltengine/bolt"
	"github.com/TeamFairmont/boltshared/config"
)

var (
	cfgdir  = "/etc/bolt" // Dir containing the customized config.json
	cfgpath bytes.Buffer  // Full path to config.json. Typically /etc/bolt/config.json
	engine  *bolt.Engine
)

func main() {
	engine = &bolt.Engine{}
	engine.Log = logrus.StandardLogger()

	// Add a slash to the end of cfgdir if needed.
	if !strings.HasSuffix(cfgdir, "/") {
		cfgdir += "/"
	}
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

}
