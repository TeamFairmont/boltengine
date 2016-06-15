// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

// Version for bolt engine and components
const Version = "1.1.0"

// EngineName is the 'friendly' marketing name of the product
const EngineName = "Bolt Engine"

// ConfigPath is the file location to check for a config.json file
const ConfigPath = "/etc/bolt/config.json"

// HaltCallCommandName is the string pased to payload.nextCommand to stop all further processing of an api call
const HaltCallCommandName = "HALT_CALL"
