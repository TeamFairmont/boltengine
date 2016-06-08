// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	//"log/syslog"

	log "github.com/Sirupsen/logrus"
	//"github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/rifflock/lfshook"
	"github.com/weekface/mgorus"
)

// CreateLogger sets up a logger instance with configed hooks, etc
func (engine *Engine) CreateLogger() {
	cfg := engine.Config
	logger := log.StandardLogger()

	switch cfg.Logging.Level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "panic":
		log.SetLevel(log.PanicLevel)
	}

	//use json format by default unless we have a blank "type"
	log.SetFormatter(&log.JSONFormatter{})

	//add hooks to external services
	switch cfg.Logging.Type {
	case "fs":
		log.AddHook(lfshook.NewHook(lfshook.PathMap{
			log.DebugLevel: cfg.Logging.FsDebugPath,
			log.InfoLevel:  cfg.Logging.FsInfoPath,
			log.WarnLevel:  cfg.Logging.FsWarnPath,
			log.ErrorLevel: cfg.Logging.FsErrorPath,
			log.FatalLevel: cfg.Logging.FsFatalPath,
			log.PanicLevel: cfg.Logging.FsPanicPath,
		}))

	case "syslog":
		//func to work around no windows syslog stubs in golang
		addSysLogHook(cfg)

	case "mongodb":
		hook, err := mgorus.NewHooker(cfg.Logging.MongoIPPort, cfg.Logging.MongoDb, cfg.Logging.MongoCollection)
		if err != nil {
			log.Panic("Log: Couldn't connect to configured mongodb server")
		}
		log.AddHook(hook)
	default:
		log.SetFormatter(&log.TextFormatter{})
	}

	engine.Log = logger
}

// LogDebug logs an info message with fields
func (engine *Engine) LogDebug(code string, fields log.Fields, message string) {
	if fields == nil {
		fields = log.Fields{}
	}
	fields["code"] = code
	engine.Log.WithFields(fields).Debug(message)
}

// LogInfo logs an info message with fields
func (engine *Engine) LogInfo(code string, fields log.Fields, message string) {
	if fields == nil {
		fields = log.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Info(message)
}

// LogWarn logs an info message with fields
func (engine *Engine) LogWarn(code string, fields log.Fields, message string) {
	if fields == nil {
		fields = log.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Warn(message)
}

// LogError logs an info message with fields
func (engine *Engine) LogError(code string, fields log.Fields, message string) {
	if fields == nil {
		fields = log.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Error(message)
}

// LogFatal logs an info message with fields
func (engine *Engine) LogFatal(code string, fields log.Fields, message string) {
	if fields == nil {
		fields = log.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Fatal(message)
}

// LogPanic logs an info message with fields
func (engine *Engine) LogPanic(code string, fields log.Fields, message string) {
	if fields == nil {
		fields = log.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Panic(message)
}
