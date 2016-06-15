// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/go-redis/cache.v1"

	"github.com/Sirupsen/logrus"
	"github.com/TeamFairmont/amqp"
	"github.com/TeamFairmont/gabs"

	"github.com/TeamFairmont/boltengine/bolterror"
	"github.com/TeamFairmont/boltengine/commandprocess"
	"github.com/TeamFairmont/boltengine/requestmanager"
	"github.com/TeamFairmont/boltengine/throttling"
	"github.com/TeamFairmont/boltshared/config"
	"github.com/TeamFairmont/boltshared/mqwrapper"
	"github.com/TeamFairmont/boltshared/stats"
	"github.com/TeamFairmont/boltshared/utils"
)

var (
	debugFormTemplate *template.Template
)

// Engine holds config info, server struct, etc for a running engine
type Engine struct {
	Config     *config.Config
	ConfigPath string

	Server        *http.Server
	Mux           *http.ServeMux
	ContextNoAuth *Context
	ContextAuth   *Context

	Log      *logrus.Logger
	Requests *requestmanager.RequestManager
	Stats    *stats.Collector
	Throttle map[string]map[int]time.Time

	mqConnection *mqwrapper.Connection
	cacheCodec   *cache.Codec

	shutdown bool //set to true when .Shutdown() is called
}

// PostConfig takes the populated config and prepares auto cors, command durations, etc.
func (engine *Engine) PostConfig(cfg *config.Config) error {

	//set engine fields
	engine.Config = cfg
	engine.CreateLogger()

	engine.Stats = stats.NewStatCollector("boltengine")
	engine.Stats.DisableTimes()

	//setup auto cors if applicable (thanks to chrome, this is needed as it sends CORS with even same-origin POSTs)
	if cfg.Security.CorsAutoAddLocal {
		prefix := "http://"
		if cfg.Engine.TLSEnabled {
			prefix = "https://"
		}
		cfg.Security.CorsDomains = append(cfg.Security.CorsDomains, prefix+"localhost")
		cfg.Security.CorsDomains = append(cfg.Security.CorsDomains, prefix+"localhost"+cfg.Engine.Bind)
		ips, err := utils.GetLocalIPs()
		if err == nil {
			for _, v := range ips {
				cfg.Security.CorsDomains = append(cfg.Security.CorsDomains, prefix+v)
				cfg.Security.CorsDomains = append(cfg.Security.CorsDomains, prefix+v+cfg.Engine.Bind)
			}
		}
	}

	engine.LogInfo("config", logrus.Fields{"cors": cfg.Security.CorsDomains}, "")

	//prep command durations, etc (wouldn't it be nice if json.Unmarshall would auto-parse right into a time.Duration?)
	for k, v := range cfg.APICalls {
		for i := 0; i < len(v.Commands); i++ {
			v.Commands[i].ResultTimeout = time.Duration(v.Commands[i].ResultTimeoutMs) * time.Millisecond

			ccfg, err := gabs.ParseJSON(v.Commands[i].ConfigParams)
			if err != nil {
				engine.LogError("init", logrus.Fields{"error": err}, "CustomizeConfig error in configParams")
				engine.Config = cfg
				return err
			}
			v.Commands[i].ConfigParamsObj = ccfg
		}
		v.Cache.ExpirationTime = time.Duration(v.Cache.ExpirationTimeSec) * time.Second
		v.ResultTimeout = time.Duration(v.ResultTimeoutMs) * time.Millisecond
		v.ResultZombie = time.Duration(v.ResultZombieMs) * time.Millisecond

		cfg.APICalls[k] = v
	}

	//set auth mode
	switch cfg.Engine.AuthMode {
	case "hmac":
		cfg.Engine.AuthModeValue = config.AuthModeHMAC
		break
	case "simple":
		cfg.Engine.AuthModeValue = config.AuthModeSimple
	}

	//parse worker config
	wcfg, err := gabs.ParseJSON(engine.Config.WorkerConfig)
	if err != nil {
		engine.LogError("init", logrus.Fields{"error": err}, "CustomizeConfig error in workerConfig")
		engine.Config = cfg
		return err
	}
	engine.Config.WorkerConfigObj = wcfg

	engine.Mux = http.NewServeMux()

	return nil
}

// ListenAndServe performs final configs and starts the http server listening with the config port and https certs
func (engine *Engine) ListenAndServe() error {
	//read debug form
	var err error
	debugFormTemplate, err = template.ParseFiles("./html_static/debugForm.tpl")
	if debugFormTemplate == nil || err != nil {
		engine.LogWarn("init", logrus.Fields{"error": err}, "debugForm.tpl not found or error loading.")
	}

	//Initialize throttling for all the groups
	engine.Throttle = throttle.InitThrottleGroups(&engine.Config.Security.Groups, engine.Throttle)

	//create request manager
	engine.Requests = requestmanager.NewRequestManager()

	if engine.Config != nil {
		// start expiring results, stat log
		go engine.expireResults()
		go engine.logStats()

		// Start the webservice
		readTimeout, err := time.ParseDuration(engine.Config.Engine.Advanced.ReadTimeout)
		if err != nil {
			engine.LogFatal("start", logrus.Fields{
				"err": err,
			}, "Invalid duration format: readTimeout")
		}
		writeTimeout, err := time.ParseDuration(engine.Config.Engine.Advanced.WriteTimeout)
		if err != nil {
			engine.LogFatal("start", logrus.Fields{
				"err": err,
			}, "Invalid duration format: writeTimeout")
		}

		maxbytes := engine.Config.Engine.Advanced.MaxHTTPHeaderKBytes << 10 // in kb

		engine.Server = &http.Server{
			Addr:           engine.Config.Engine.Bind,
			ReadTimeout:    readTimeout,
			WriteTimeout:   writeTimeout,
			MaxHeaderBytes: maxbytes, // 1024 << 10 = 1 MB
			Handler:        engine.Mux,
		}

		//connect to MQ
		engine.mqConnection, err = mqwrapper.ConnectMQ(engine.Config.Engine.MQUrl)
		if err != nil {
			engine.LogFatal("start", logrus.Fields{
				"err": err,
			}, "Couldn't connect to mqUrl")
		} else {
			// recoverMqConnection also sets up the worker error queue goroutine
			go engine.recoverMqConnection()
		}
		defer engine.mqConnection.Close()

		//setup cache
		err = engine.SetupCache()
		if err != nil {
			engine.LogFatal("start", logrus.Fields{
				"err": err,
			}, "Couldn't connect to cache")
		}

		//check if stubMode is active
		if engine.Config.Engine.Advanced.StubMode {
			go engine.workerStub()
		}

		// Intercept signal notifications for clean shutdown
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			for sig := range signalChan {
				engine.LogInfo("os_interrupt", logrus.Fields{"signal": sig}, "OS Signal Received")
				engine.Shutdown()
			}
		}()

		//finally start
		engine.Stats.Ch("general").Ch("engine_start").V(time.Now())
		engine.LogInfo("start", logrus.Fields{
			"version":     Version,
			"bind":        utils.GetLocalIP() + engine.Config.Engine.Bind,
			"authMode":    engine.Config.Engine.AuthMode,
			"queuePrefix": engine.Config.Engine.Advanced.QueuePrefix,
		}, EngineName+" Started")

		if engine.Config.Engine.TLSEnabled {
			// Start the TLSed engine services
			engine.LogFatal("start", logrus.Fields{
				"engine.Server.ListenAndServeTLS()": engine.Server.ListenAndServeTLS(engine.Config.Engine.TLSCertFile, engine.Config.Engine.TLSKeyFile),
			}, "ListenAndServe()")
		} else {
			engine.LogWarn("start", logrus.Fields{
				"version": Version,
				"bind":    utils.GetLocalIP() + engine.Config.Engine.Bind,
			}, "NOT running over https! Use tlsEnabled before going to production")

			// Start the non-TLSed engine services
			engine.LogFatal("start", logrus.Fields{
				"engine.Server.ListenAndServe()": engine.Server.ListenAndServe(),
			}, "ListenAndServe()")
		}

		return nil
	}
	return errors.New("No config loaded")
}

// CreateTestEngine returns a fully configured engine with a special test config to be used
// in unit tests, etc.
// go ListenAndServe() must be called by the test if needed.
// -- DO NOT USE IN PRODUCTION CODE --
func CreateTestEngine(loglevel string) *Engine {
	engine := &Engine{}
	engine.Log = logrus.StandardLogger()

	cfg, err := config.DefaultConfig()
	cfg, err = config.CustomizeConfig(cfg, config.TestConfigJSON)
	if err != nil {
		return nil
	}
	cfg.Logging.Level = loglevel
	engine.PostConfig(cfg)
	BuiltinHandlers(engine)

	//if engine.ListenAndServe() != nil {
	//	engine.LogFatal("critical", nil, "Error starting server, check config and ports")
	//}
	return engine
}

// Shutdown sets the engine to reject new non-core API calls, and waits for
// current commands to timeout or complete before exiting. Returns false if
// a shutdown process was already started, true otherwise
func (engine *Engine) Shutdown() bool {
	if !engine.shutdown {
		engine.LogInfo("shutdown", logrus.Fields{"count": engine.Requests.Count()}, "Shutdown started.")
		startTime := time.Now()
		engine.shutdown = true
		go func() {
			d, _ := time.ParseDuration("5s")
			shutdownResultExpiration, err := time.ParseDuration(engine.Config.Engine.Advanced.ShutdownResultExpiration)
			if err != nil {
				engine.LogError("config", nil, "engine.advanced.shutdownResultExpiration error: couldn't parse. Using 30s as default")
				shutdownResultExpiration, _ = time.ParseDuration("30s")
			}
			forceQuit, err := time.ParseDuration(engine.Config.Engine.Advanced.ShutdownForceQuit)
			if err != nil {
				engine.LogError("config", nil, "engine.advanced.shutdownForceQuit error: couldn't parse. Using 120s as default")
				forceQuit, _ = time.ParseDuration("120s")
			}

			//loop, wait for expiring results
			for {
				exp := engine.Requests.ExpireCompletedRequests(shutdownResultExpiration)
				engine.LogInfo("shutdown", logrus.Fields{"count": engine.Requests.Count(), "expired": len(exp)}, "Shutdown in progress...")
				if engine.Requests.Count() > 0 {
					diff := time.Now().Sub(startTime)
					if diff >= forceQuit {
						engine.LogWarn("shutdown", logrus.Fields{"count": engine.Requests.Count()}, "Shutdown before all requests complete or expired")
						os.Exit(0)
					}
					time.Sleep(d)
				} else {
					engine.LogInfo("shutdown", nil, "Shutdown complete")
					os.Exit(0)
				}
			}
		}()
		return true
	}
	return false
}

// IsShutdown returns true if engine is in shutdown mode
func (engine *Engine) IsShutdown() bool {
	return engine.shutdown
}

// workerErrorQueue logs any messages received via the Prefix+ErrorQueue
func (engine *Engine) workerErrorQueue(reconnectsig chan bool) {
	//engine.Config.Engine.Advanced.QueuePrefix+engine.Config.Engine.Advanced.ErrorQueue
	_, res, err := mqwrapper.CreateConsumeNamedQueue(engine.Config.Engine.Advanced.QueuePrefix+config.ErrorQueueName, engine.mqConnection.Channel)
	if err != nil {
		engine.LogWarn("worker_log", logrus.Fields{"error": err}, "Could not connect to worker error queue")
	} else {

		for {
			select {
			case <-reconnectsig:
				_, res, err = mqwrapper.CreateConsumeNamedQueue(engine.Config.Engine.Advanced.QueuePrefix+config.ErrorQueueName, engine.mqConnection.Channel)
				if err != nil {
					engine.LogWarn("worker_log", logrus.Fields{"error": err}, "Could not connect to worker error queue")
				}

			case d := <-res:
				if d.RoutingKey != "" {
					engine.LogWarn("worker_log", logrus.Fields{"data": string(d.Body)}, "")
				}
				d.Ack(false)
			}

		}
	}
}

// recoverMqConnection registers and monitors the mq disconnect event and tries to
// re-establish connection on a disconnect or error
func (engine *Engine) recoverMqConnection() {
	reconnectsig := make(chan bool)
	go engine.workerErrorQueue(reconnectsig)

	disc := engine.mqConnection.Connection.NotifyClose(make(chan *amqp.Error))
	d, _ := time.ParseDuration("1s")
	go func() {
		for {
		DisconnectEvent:
			for ev := range disc {
				engine.LogDebug("mq_ev", logrus.Fields{"ev": ev}, "Range disc")
				for { //enter reconnect loop
					engine.LogWarn("mq_error", logrus.Fields{}, "Attempt reconnect to mqUrl")
					var err error
					engine.mqConnection, err = mqwrapper.ConnectMQ(engine.Config.Engine.MQUrl)
					if err != nil {
						engine.LogError("mq_error", logrus.Fields{"err": err}, "Couldn't reconnect to mqUrl")
						time.Sleep(d)
					} else {
						engine.LogInfo("mq_connect", logrus.Fields{}, "Successfully reconnected to mqUrl")
						reconnectsig <- true
						defer engine.mqConnection.Close()
						disc = engine.mqConnection.Connection.NotifyClose(make(chan *amqp.Error))
						break DisconnectEvent
					}
				}
			}
		}
	}()
}

// expireResults periodically clears all completed results from the request manager
func (engine *Engine) expireResults() {

	d, err := time.ParseDuration(engine.Config.Engine.Advanced.CompleteResultLoopFreq)
	if err != nil {
		engine.LogError("config", nil, "engine.advanced.completeResultLoopFreq error: couldn't parse. Using 10s as default")
		d, _ = time.ParseDuration("10s")
	}

	completeResultExpiration, err := time.ParseDuration(engine.Config.Engine.Advanced.CompleteResultExpiration)
	if err != nil {
		engine.LogError("config", nil, "engine.advanced.completeResultExpiration error: couldn't parse. Using 10s as default")
		completeResultExpiration, _ = time.ParseDuration("10s")
	}

	for {
		time.Sleep(d)
		if !engine.IsShutdown() {
			expired := engine.Requests.ExpireCompletedRequests(completeResultExpiration)
			for id := range expired {
				engine.LogInfo("call_expired", logrus.Fields{"id": expired[id]}, "")
				engine.Stats.Ch("general").Ch("expired_results").Incr()
			}
		}
	}
}

// logStats periodically writes the stat json to info log
func (engine *Engine) logStats() {
	d, _ := time.ParseDuration("600s") //default 10 minutes, TODO: config var
	for {
		time.Sleep(d)
		json, _ := engine.Stats.JSON()
		engine.LogInfo("stats", logrus.Fields{}, json)
	}
}

// OutputError outputs an error message and logs it using the engine's log system
func (engine *Engine) OutputError(w http.ResponseWriter, err *bolterror.BoltError) {
	engine.LogDebug("call_error", logrus.Fields{"bolterror": err}, "Error with API call")
	OutputError(w, err)
}

//*************************
// 'static' functions begin
//*************************

//For use in ExtractCommandName
var flags = []string{"/request/",
	"/task/",
	"/work/",
	"/form/",
}

// ExtractCallName pulls the API call name from the URL and returns the string.
func ExtractCallName(r *http.Request) (string, error) {

	//loops through the array of flags
	//If the beginning of the URL.Path matches a flag exactly
	//remove the flag and return the remaining URL.Path and a nil error
	for _, flag := range flags {
		if len(r.URL.Path) >= len(flag) && r.URL.Path[:len(flag)] == flag {
			return r.URL.Path[len(flag):], nil
		}
	} //if it matches nothing it is an error
	return r.URL.Path, errors.New("Invalid flag, couldn't extract call")
}

// OutputError writes a JSON error response to w with no other outputs params or variables
func OutputError(w http.ResponseWriter, err *bolterror.BoltError) {
	g, _ := gabs.ParseJSON([]byte("{}"))
	err.AddToPayload(g)
	fmt.Fprintf(w, "%s", g.String())
}

// OutputRequest filters the payload and writes a call request to w
func (engine *Engine) OutputRequest(w http.ResponseWriter, req *commandprocess.CommandProcess, filterKeys []string) {
	ret := ""
	var payload *gabs.Container
	var err error
	//if filter keys is nil, do not filter
	if filterKeys != nil {
		payload, err = utils.FilterPayload(req.Payload, filterKeys)
	} else {
		payload = req.Payload
	}
	if err != nil {
		engine.LogError("OutputRequest", logrus.Fields{"ev": err}, "Filter Payload Error")
	}
	if engine.Config.Engine.PrettyOutput {
		ret = payload.StringIndent("\n", "\t")
	} else {
		ret = payload.String()
	}

	fmt.Fprint(w, ret)
}

type debugFormFields struct {
	CommandName    string
	CommandInfo    *config.APICall
	RequiredParams string
}

// OutputDebugForm spits out the debug form template for /form/
func (engine *Engine) OutputDebugForm(w http.ResponseWriter, r *http.Request) {
	cmd, _ := ExtractCallName(r)

	w.Header().Set("Content-Type", "text/html")

	if debugFormTemplate != nil {

		apicall, ok := engine.Config.APICalls[cmd]
		if !ok {
			//engine.OutputError(w, bolterror.NewBoltError(nil, "request", "Unknown API Call", cmd, bolterror.Request))
			debugFormTemplate.Execute(w, debugFormFields{CommandName: "(Unknown API Call)", CommandInfo: nil})
			return
		}
		reqParams, err := json.MarshalIndent(apicall.RequiredParams, "", "\t")
		if err != nil {
			reqParams = []byte("")
		}
		debugFormTemplate.Execute(w, debugFormFields{CommandName: cmd, CommandInfo: &apicall, RequiredParams: string(reqParams)})
	}
}
