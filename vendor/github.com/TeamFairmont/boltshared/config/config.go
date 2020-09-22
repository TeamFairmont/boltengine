// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config provides the default configuration for the Bolt engine.  It also provides
// a method for overriding any default settings with a custom json string.
//
// The config can be referenced as shown in these examples:
//		engine.LogInfo("config", nil, "engine > version =" + cfg.Engine.Version)
//		engine.LogInfo("config", nil, "workerConfig > primaryDb > host =" + cfg.WorkerConfig.PrimaryDb.Host)
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/TeamFairmont/gabs"
	"github.com/xeipuuv/gojsonschema"
)

// AuthMode constants for mode of security verification
const (
	AuthModeHMAC = iota
	AuthModeSimple
)

// Config structure created from the config_defauls.json file using jsonutils
// Website- https://github.com/bashtian/jsonutils
// go get github.com/bashtian/jsonutils/cmd/jsonutil
// jsonutil -x -c=false -f /etc/bolt/config.json
type Config struct {
	Engine struct {
		Version           string `json:"version"`     // v1
		Bind              string `json:"bind"`        // :443
		TLSCertFile       string `json:"tlsCertFile"` // cert.pem
		TLSKeyFile        string `json:"tlsKeyFile"`  // key.pem
		TLSEnabled        bool   `json:"tlsEnabled"`  // false
		AuthMode          string `json:"authMode"`    // "hmac" (or "simple")
		AuthModeValue     int    `json:"-"`
		MQUrl             string `json:"mqUrl"`             //amqp://guest:guest@localhost:5672/
		PrettyOutput      bool   `json:"prettyOutput"`      //false (true enabled indented and line breaks in JSON responses)
		ExtraConfigFolder string `json:"extraConfigFolder"` // "etc/bolt"
		TraceEnabled      bool   `json:"traceEnabled"`      //true (enables per-command trace output in api calls)
		DocsEnabled       bool   `json:"docsEnabled"`
		Advanced          struct {
			ReadTimeout              string `json:"readTimeout"`              //30s
			WriteTimeout             string `json:"writeTimeout"`             //30s
			CompleteResultLoopFreq   string `json:"completeResultLoopFreq"`   // 10s
			CompleteResultExpiration string `json:"completeResultExpiration"` // 30s
			ShutdownResultExpiration string `json:"shutdownResultExpiration"` // 30s
			ShutdownForceQuit        string `json:"shutdownForceQuit"`        // 120s
			StubMode                 bool   `json:"stubMode"`                 //false (true enables spining up a 'stub' worker for all config'ed commands)
			StubDelayMs              int64  `json:"stubDelayMs"`              //100
			DebugFormEnabled         bool   `json:"debugFormEnabled"`         // true
			MaxHTTPHeaderKBytes      int    `json:"maxHTTPHeaderKBytes"`      // 0 (default go http lib makes it 1MB)
			QueuePrefix              string `json:"queuePrefix"`              // "" default, this string will be prefixed on the queue for every command name
		} `json:"advanced"`
	} `json:"engine"`

	Logging struct {
		Type  string `json:"type"`  //fs, syslog, mongodb (or add more in bolt/logging.go)
		Level string `json:"level"` //debug, info, warn, error, fatal, panic

		LogStatsDuration string `json:"logStatsDuration"` // for logstats

		//fs options
		FsDebugPath string `json:"fsDebugPath"`
		FsInfoPath  string `json:"fsInfoPath"`
		FsWarnPath  string `json:"fsWarnPath"`
		FsErrorPath string `json:"fsErrorPath"`
		FsFatalPath string `json:"fsFatalPath"`
		FsPanicPath string `json:"fsPanicPath"`

		//syslog options
		SyslogProtocol string `json:"syslogProtocol"` //udp
		SyslogIPPort   string `json:"syslogIPPort"`   //localhost:445

		//mongodb options
		MongoIPPort     string `json:"mongoIPPort"`     //localhost:27017
		MongoDb         string `json:"mongoDb"`         //db
		MongoCollection string `json:"mongoCollection"` //collection
	} `json:"logging"`

	Security struct {
		VerifyTimeout    int64            `json:"verifyTimeout"` //30
		Groups           []SecurityGroups `json:"groups"`
		HandlerAccess    []HandlerAccess  `json:"handlerAccess"`    //[]
		CorsDomains      []string         `json:"corsDomains"`      //[]
		CorsAutoAddLocal bool             `json:"corsAutoAddLocal"` //true
	} `json:"security"`

	Cache struct {
		Type      string `json:"type"`      // redis
		Host      string `json:"host"`      // localhost:1234
		Pass      string `json:"pass"`      //
		TimeoutMs int64  `json:"timeoutMs"` // 2000
	} `json:"cache"`

	APICalls        map[string]APICall     `json:"apiCalls"`
	CommandMetas    map[string]CommandMeta `json:"commandMeta"`
	WorkerConfig    json.RawMessage        `json:"workerConfig"`
	WorkerConfigObj *gabs.Container        `json:"-"`
}

// SecurityGroups holds group names and their corresponding HMAC keys
type SecurityGroups struct {
	Name              string `json:"name"`              // readonly
	Hmackey           string `json:"hmackey"`           // N9d*22UuzdA443Nur2eL23:a2fvTqe
	RequestsPerSecond int64  `json:"requestsPerSecond"` // 0
}

// HandlerAccess holds handler names and arrays of groups to either deny or allow access.
type HandlerAccess struct {
	HandlerURL string `json:"handler"` // full url of handler or APICall to limit access - ex: /work/v1/addProduct, or /pending.  Checked for exact match to request url in context.go
	APICall    string `json:"apiCall"` // api call name only. ie v1/addProduct.  Checked as prefix to request url in context.go
	//if either handlerURL or apiCall is matched and non-blank string, the below access rules will apply
	DenyGroups  []string `json:"denyGroups"`  //[]
	AllowGroups []string `json:"allowGroups"` //[]
}

// CommandMeta is a simple holder for additional params common to each possible command
type CommandMeta struct {
	RequiredParams   map[string]string `json:"requiredParams"`
	NoStub           bool              `json:"noStub"`           // false (if true, then in stubMode engine won't generate a stub for this command)
	StubReturn       json.RawMessage   `json:"stubReturn"`       // empty, if populated, then in stubMode this message will be added to return_value in payload
	StubData         json.RawMessage   `json:"stubData"`         // empty, if populated, then in stubMode this message will be added to data in payload
	StubDelayMs      int64             `json:"stubDelayMs"`      // 0, if non-zero, then in stubMode this command will use this delay instead of the default
	LongDescription  string            `json:"longDescription"`  // Expanded description
	ShortDescription string            `json:"shortDescription"` // Brief description
}

// CommandInfo stores command details and config within an API call
type CommandInfo struct {
	Name            string          `json:"name"`            // product/checkDuplicates
	ResultTimeoutMs int64           `json:"resultTimeoutMs"` // 6000
	ResultTimeout   time.Duration   `json:"-"`
	ReturnAfter     bool            `json:"returnAfter"` // false
	ConfigParams    json.RawMessage `json:"configParams"`
	ConfigParamsObj *gabs.Container `json:"-"`
}

// APICall holds performance config, required params, etc for an entire API call and
// is itself the definition for the consumable API
type APICall struct {
	ResultTimeoutMs int64         `json:"resultTimeoutMs"` // 100
	ResultTimeout   time.Duration `json:"-"`               //time to wait for the total call to complete before returning a timeout + id

	ResultZombieMs int64         `json:"resultZombieMs"` // 30000
	ResultZombie   time.Duration `json:"-"`              //time to wait between commands completing to consider this call 'zombied'

	Cache struct {
		Enabled           bool          `json:"enabled"`           // false
		ExpirationTimeSec int64         `json:"expirationTimeSec"` // 600
		ExpirationTime    time.Duration `json:"-"`
	} `json:"cache"`

	RequiredParams map[string]string `json:"requiredParams"`

	Commands         []CommandInfo `json:"commands"`
	FilterKeys       []string      `json:"filterKeys"`
	LongDescription  string        `json:"longDescription"`  // Expandable description
	ShortDescription string        `json:"shortDescription"` // Brief description
}

// JSON outputs the config struct as a JSON string
func (cfg *Config) JSON() (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DefaultConfig returns a default configuration variable to the caller.
func DefaultConfig() (*Config, error) {
	var defaults = `{
		"engine": {
	  		"version": "v1",
			"bind": ":443",
			"tlsCertFile":"/etc/bolt/cert.pem",
			"tlsKeyFile":"/etc/bolt/key.pem",
			"tlsEnabled": true,
			"authMode": "hmac",
			"mqUrl":"amqp://guest:guest@localhost:5672/",
			"extraConfigFolder": "etc/bolt/",
			"prettyOutput": false,
			"traceEnabled": true,
			"docsEnabled": true,
			"advanced": {
				"completeResultExpiration": "30s",
				"completeResultLoopFreq": "10s",
				"debugFormEnabled": false,
				"maxHTTPHeaderKBytes": 1024,
				"readTimeout":"30s",
				"shutdownForceQuit": "120s",
				"shutdownResultExpiration": "30s",
				"stubDelayMs": 100,
				"stubMode": false,
				"writeTimeout":"30s",
                "queuePrefix":""
			}
		},

		"logging": {
			"type": "",
			"level": "debug"
		},

		"security": {
			"verifyTimeout": 30,
			"groups": [],
			"corsDomains": [],
			"corsAutoAddLocal": true
		},

		"cache": {
			"type": "",
			"host": "localhost:6379",
			"pass": "",
			"timeoutMs": 2000
		},

		"workerConfig": {},

		"apiCalls": {},

		"commandMeta": {}
	}`

	// Unmarshal the default json string into an interface.
	var defaultConfig map[string]interface{}
	if err := json.Unmarshal([]byte(defaults), &defaultConfig); err != nil {
		return nil, err
	}

	// Create a new variable of the Config struct and populate it with the default values.
	config := &Config{}
	if err := json.Unmarshal([]byte(defaults), &config); err != nil {
		return nil, err
	}
	return config, nil
}

// CustomizeConfig takes an existing Config (usually defaults) and a string of json (usually the client's custom config settings).
func CustomizeConfig(config *Config, custom string) (*Config, error) {
	if err := json.Unmarshal([]byte(custom), &config); err != nil {
		return nil, err
	}
	return config, nil
}

// BuildConfig creates an engine config by first reading the default config, then overriding it with the contents of config.json
// cfgdir: Directory containing the customized config.json - Typically: "/etc/bolt/"
// cfgpath: Full path to config.json - Typically: "/etc/bolt/config.json"
func BuildConfig(cfgdir, cfgpath string) (*Config, error) {
	var errbuf bytes.Buffer
	var fileConfigPath bytes.Buffer

	// Create a default config
	defaultcfg, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	// Overwrite the default config with the json created by reading the client's config.json file.
	// If it doesn't exist, use the version in etc/bolt/config.json
	customcfg := defaultcfg
	usecustom := true
	fileConfigPath.WriteString("file://")
	clientconfig, err := ioutil.ReadFile(cfgpath)
	if err != nil {
		// An error here means the custom config file doesn't exist.
		// Use the default config instead in etc/bolt/config.json
		clientconfig, err = ioutil.ReadFile("etc/bolt/config.json")
		usecustom = false
		if err != nil {
			return nil, err
		}
		fileConfigPath.WriteString("etc/bolt/")
	} else {
		fileConfigPath.WriteString(cfgdir)
	}
	fileConfigPath.WriteString("config.json") // "file:///etc/bolt/config.json"

	// Validate the config json against the schema
	schemaLoader := gojsonschema.NewStringLoader(SCHEMA)
	documentLoader := gojsonschema.NewReferenceLoader(fileConfigPath.String())
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, err
	}
	if !result.Valid() {
		errbuf.WriteString("Invalid config.json")
		for _, desc := range result.Errors() {
			errbuf.WriteString("\nJSON Schema Issue- ")
			errbuf.WriteString(fmt.Sprintf("%s", desc))
		}
		errbuf.WriteString("\n")
		return nil, errors.New(errbuf.String())
	}

	customcfg, err = CustomizeConfig(defaultcfg, string(clientconfig))
	if err != nil {
		// Return the error, including the path to the file with the error.
		// e.g. invalid character ',' looking for beginning of object key string in /etc/bolt/config.json
		errbuf.WriteString(err.Error())
		errbuf.WriteString(" in ")
		if usecustom {
			errbuf.WriteString("custom ")
			errbuf.WriteString(cfgpath)
		} else {
			errbuf.WriteString("default ")
			errbuf.WriteString("etc/bolt/config.json")
		}
		err = errors.New(errbuf.String())
		return nil, err
	}

	customcfg, err = loadIndividualConfigs(customcfg)
	if err != nil {
		return nil, err
	}

	// All done.  Return the customized config.
	return customcfg, nil
}

// loadIndividualConfigs overwrites the existing config branches with the contents of the individual json files.
// The schema of each json file is validated.
func loadIndividualConfigs(customcfg *Config) (*Config, error) {
	var errbuf bytes.Buffer
	var extraConfigFolder string

	// Determine if the client's ExtraConfigFolder ends with a slash.  If not, add one.
	if !strings.HasSuffix(customcfg.Engine.ExtraConfigFolder, "/") {
		var appendBuffer bytes.Buffer
		appendBuffer.WriteString(customcfg.Engine.ExtraConfigFolder)
		appendBuffer.WriteString("/")
		customcfg.Engine.ExtraConfigFolder = appendBuffer.String()
	}
	// For loading extra configs, only use the path set in config.json.  This ignore engine.json's extraConfigFolder.
	extraConfigFolder = customcfg.Engine.ExtraConfigFolder

	// Load each of the custom json files, if they exist, in the path specified in engine.extraConfigFolder
	// The folder for additional config files can be specified in config.json - engine > extraConfigFolder
	cfgJSON := []string{
		"apiCalls",
		"cache",
		"commandMeta",
		"engine",
		"logging",
		"security",
		"workerConfig",
	}

	// Load the schema to check the individual json against
	schemaLoader := gojsonschema.NewStringLoader(SCHEMA)

	// Loop through the list of possible files
	for i := 0; i < len(cfgJSON); i++ {
		// Get the path to the custom config files and read them (if they exist)
		var path bytes.Buffer
		path.WriteString(extraConfigFolder)
		path.WriteString(cfgJSON[i])
		path.WriteString(".json")

		// Overwrite the branches of config with the contents of any existing custom config files.
		// If an individual file doesn't exist in the cfgpath, skip it.
		_, err := os.Stat(path.String())
		if err == nil {
			// The file exists.  Write its contents to a buffer.
			var cfgbuf bytes.Buffer
			cfgbuf.WriteString("{\"")
			cfgbuf.WriteString(cfgJSON[i])
			cfgbuf.WriteString("\": ")
			readconfig, err := ioutil.ReadFile(path.String())
			if err != nil {
				return nil, err
			}
			cfgbuf.WriteString(string(readconfig))
			cfgbuf.WriteString("}")

			// Validate the buffer's custom json string against the schema
			documentLoader := gojsonschema.NewStringLoader(cfgbuf.String())
			result, err := gojsonschema.Validate(schemaLoader, documentLoader)
			if err != nil {
				return nil, err
			}
			if !result.Valid() {
				errbuf.WriteString("Invalid ")
				errbuf.WriteString(path.String())
				for _, desc := range result.Errors() {
					errbuf.WriteString("\nJSON Schema Issue- ")
					errbuf.WriteString(fmt.Sprintf("%s", desc))
				}
				errbuf.WriteString("\n")
				return nil, errors.New(errbuf.String())
			}

			// Replace existing values for this branch of the config with the contents of the buffer.
			customcfg, err = CustomizeConfig(customcfg, cfgbuf.String())
			if err != nil {
				// Return the error, including the path to the file with the error.
				// e.g. invalid character ',' looking for beginning of object key string in /etc/bolt/apiCalls.json
				var errbuf bytes.Buffer
				errbuf.WriteString(err.Error())
				errbuf.WriteString(" in ")
				errbuf.WriteString(path.String())
				err = errors.New(errbuf.String())
				return nil, err
			}
		}
	}
	return customcfg, nil

}
