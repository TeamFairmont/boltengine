// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build !windows

package bolt

import (
	"log/syslog"

	"github.com/TeamFairmont/boltshared/config"
	"github.com/sirupsen/logrus"
	logrus_syslog "github.com/sirupsen/logrus/hooks/syslog"
)

func addSysLogHook(cfg *config.Config) {
	hook, err := logrus_syslog.NewSyslogHook(cfg.Logging.SyslogProtocol, cfg.Logging.SyslogIPPort, syslog.LOG_INFO, "")
	if err != nil {
		logrus.Panic("Log: Couldn't connect to configured syslog daemon")
	}
	logrus.AddHook(hook)
}
