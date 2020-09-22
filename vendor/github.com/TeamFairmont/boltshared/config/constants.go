// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// ErrorQueueName is the name of the mq (minus prefix) where the engine waits for worker errors to log
const ErrorQueueName = "BOLT_WORKER_ERROR"

// TestConfigJSON is a barebones engine config to be used
// by CreateTestEngine in unit tests
const TestConfigJSON = `{
    "engine": {
	  		"version": "TEST CONFIG",
			"bind": ":8888",
			"tlsEnabled": false,
			"authMode": "hmac",
			"mqUrl":"amqp://guest:guest@localhost:5672/",
			"extraConfigFolder": "",
			"prettyOutput": false,
			"traceEnabled": true,
			"docsEnabled": true,
			"advanced": {
				"stubMode": false,
				"stubDelay": 100
			}
		},

		"logging": {
			"type": "",
			"level": "debug"
		},

		"security": {
			"verifyTimeout": 30,
			"groups": [{
                "name": "normal",
                "hmackey": "normal"
            }, {
                "name": "throttled",
                "hmackey": "throttled",                
                "requestsPerSecond": 3
            }, {
                "name": "engineadmin",
                "hmackey": "engineadmin"
            }],
            "handlerAccess": [{
                "handler": "/get-config",
                "allowGroups": ["engineadmin"]
                }, {
                "handler": "/pending",
                "allowGroups":  ["engineadmin","normal"]
                }, {
                "handler": "/save-config",
                "allowGroups": ["engineadmin"]
                }, {
                "handler": "/v1/test",
                "denyGroups": ["throttled"]
                },{
                "handler": "/debug-log",
                "allowGroups": ["engineadmin"]
            }],
			"corsDomains": [],
			"corsAutoAddLocal": true
		},

		"cache": {
			"type": "redis",
			"host": "localhost:6379",
			"pass": "",
			"timeoutMs": 2000
		},

		"apiCalls": {
            "v1/test": {
                "resultTimeoutMs": 500,
                "cache": {
                    "enabled": true,
                    "expirationTimeSec": 2000
                },
                "requiredParams": {
                    "testinput": "string"
                },
                "commands": [{
                    "name": "test/command1",
                    "resultTimeoutMs": 500,
                    "returnAfter": false,
                    "configParams": {
                        "testparam": 2
                    }
                }, {
                    "name": "test/command2",
                    "resultTimeoutMs": 500,
                    "returnAfter": false,
                    "configParams": {
                        "testparam": "test"
                    }
                }, {
                    "name": "test/command3",
                    "resultTimeoutMs": 500,
                    "returnAfter": true,
                    "configParams": {
                        "testparam": true
                    }
                }],
                "longDescription":  "This is a test call from the test config",
                "shortDescription": "This is a test call"
            }
        },

		"commandMeta": {
            "test/command1": {
                "requiredParams": {
                    "testinput": "string"
                },
                "stubData": {
                    "abc": 123,
                    "def": [4, 5, 6],
                    "ghi": {
                        "val": 789
                    }
                },
                "longDescription":  "",
                "shortDescription": ""
            },
            "test/command2": {},
            "test/command3": {}
        }
}`
