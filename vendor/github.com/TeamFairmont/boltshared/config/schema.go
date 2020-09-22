// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

const SCHEMA string = `
    {
        "$schema": "http://json-schema.org/draft-04/schema#",
        "id": "/",
        "type": "object",
        "properties": {

            "apiCalls": {
                "id": "apiCalls",
                "type": "object",
                "patternProperties": {
                    ".*": {
                        "type": "object",
                        "properties": {
                            "resultTimeoutMs": {
                                "type": "integer",
                                "minimum": 0
                            },
                            "resultZombieMs": {
                                "type": "integer",
                                "minimum": 0
                            },
                            "cache": {
                                "type": "object",
                                "properties": {
                                    "enabled": {
                                        "type": "boolean"
                                    },
                                    "expirationTimeSec": {
                                        "type": "integer",
                                        "minimum": 0
                                    }
                                }
                            },
                            "requiredParams": {
                                "type": "object",
                                "properties": {}
                            },
                            "commands": {
                                "type": "array",
                                "items": {
                                    "type": "object",
                                    "properties": {
                                        "name": {
                                            "type": "string"
                                        },
                                        "resultTimeoutMs": {
                                            "type": "integer",
                                            "minimum": 0
                                        },
                                        "returnAfter": {
                                            "type": "boolean"
                                        },
                                        "configParams": {
                                            "type": "object",
                                            "properties": {}
                                        }
                                    }
                                }
                            },
                            "longDescription": {
                                "type": "string"
                            },
                            "shortDescription": {
                                "type": "string"
                            }
                        }
                    }
                }
            },

            "cache": {
                "type": "object",
                "properties": {
                    "type": {
                        "type": "string"
                    },
                    "host": {
                        "type": "string"
                    },
                    "pass": {
                        "type": "string"
                    },
                    "timeoutMs": {
                        "type": "integer",
                        "minimum": 0
                    }
                }
            },

            "commandMeta": {
                "type": "object",
                "patternProperties": {
                    ".*": {
                        "type": "object",
                        "properties": {
                            "requiredParams": {
                                "type": "object",
                                "properties": {}
                            },
                            "stubData": {
                                "type": "object",
                                "properties": {}
                            },
                            "stubReturn": {
                                "type": "object",
                                "properties": {}
                            },
                            "stubDelayMs": {
                                "type": "integer",
                                "minimum": 0
                            },
                            "noStub": {
                                "type": "boolean"
                            },
                            "longDescription": {
                                "type": "string"
                            },
                            "shortDescription": {
                                "type": "string"
                            }
                        }
                    }
                }
            },

            "engine": {
                "type": "object",
                "properties": {
                    "version": {
                        "type": "string"
                    },
                    "bind": {
                        "type": "string",
                        "pattern": "^:"
                    },
                    "tlsCertFile": {
                        "type": "string"
                    },
                    "tlsKeyFile": {
                        "type": "string"
                    },
                    "tlsEnabled": {
                        "type": "boolean"
                    },
                    "authMode": {
                        "type": "string"
                    },
                    "mqUrl": {
                        "type": "string"
                    },
                    "prettyOutput": {
                        "type": "boolean"
                    },
                    "extraConfigFolder": {
                        "type": "string"
                    },
                    "traceEnabled": {
                        "type": "boolean"
                    },
                    "docsEnabled": {
                        "type": "boolean"
                    },
                    "advanced": {
                        "type": "object",
                        "properties": {
                            "readTimeout": {
                                "type": "string"
                            },
                            "writeTimeout": {
                                "type": "string"
                            },
                            "shutdownResultExpiration": {
                                "type": "string"
                            },
                            "shutdownForceQuit": {
                                "type": "string"
                            },
                            "maxHTTPHeaderKBytes": {
                                "type": "integer",
                                "minimum": 0
                            },
                            "stubMode": {
                                "type": "boolean"
                            },
                            "stubDelayMs": {
                                "type": "integer",
                                "minimum": 0
                            },
                            "completeResultExpiration": {
                                "type": "string"
                            },
                            "completeResultLoopFreq": {
                                "type": "string"
                            },
                            "debugFormEnabled": {
                                "type": "boolean"
                            },
                            "queuePrefix": {
                                "type": "string"
                            } 
                        }
                    }
                }
            },

            "logging": {
                "type": "object",
                "properties": {
                    "level": {
                        "type": "string"
                    },
                    "logStatsDuration":{
                        "type": "string"
                    },
                    "fsDebugPath": {
                        "type": "string"
                    },
                    "fsInfoPath": {
                        "type": "string"
                    },
                    "fsWarnPath": {
                        "type": "string"
                    },
                    "fsErrorPath": {
                        "type": "string"
                    },
                    "fsFatalPath": {
                        "type": "string"
                    },
                    "fsPanicPath": {
                        "type": "string"
                    },
                    "syslogProtocol": {
                        "type": "string"
                    },
                    "syslogIPPort": {
                        "type": "string"
                    },
                    "mongoIPPort": {
                        "type": "string"
                    },
                    "mongoDb": {
                        "type": "string"
                    },
                    "mongoCollection": {
                        "type": "string"
                    }
                }
            },

            "security": {
                "type": "object",
                "properties": {
                    "verifyTimeout": {
                        "type": "integer",
                        "minimum": 0
                    },
                    "groups": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "name": {
                                    "type": "string"
                                },
                                "hmackey": {
                                    "type": "string"
                                },
                                "requestsPerSecond": {
                                    "type": "integer",
                                    "minimum": 0
                                }
                            }
                        }
                    },
                    "handlerAccess": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "handler": {
                                    "type": "string"
                                },
                                "allowGroups": {
                                    "type": "array",
                                    "items": {
                                        "type": "string"
                                    }
                                },
                                "denyGroups": {
                                    "type": "array",
                                    "items": {
                                        "type": "string"
                                    }
                                }
                            }
                        }
                    },
                    "corsDomains": {
                        "type": "array",
                        "items": {
                            "type": "string"
                        }
                    },
                    "corsAutoAddLocal": {
                        "type": "boolean"
                    }
                }
            },

            "workerConfig": {
                "type": "object",
                "properties": {}
            }
        }
    }
`
