// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"github.com/TeamFairmont/boltshared/config"
	"github.com/sirupsen/logrus"
)

func addSysLogHook(cfg *config.Config) {
	logrus.Warnln("Windows does NOT support syslog logging mode!")
	return
}
