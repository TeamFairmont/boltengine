// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/. 

// +build !windows

package bolt

import (
	"log/syslog"

	log "github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/TeamFairmont/boltshared/config"
)

func addSysLogHook(cfg *config.Config) {
	hook, err := logrus_syslog.NewSyslogHook(cfg.Logging.SyslogProtocol, cfg.Logging.SyslogIPPort, syslog.LOG_INFO, "")
	if err != nil {
		log.Panic("Log: Couldn't connect to configured syslog daemon")
	}
	log.AddHook(hook)
}
