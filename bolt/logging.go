// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bolt

import (
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"github.com/weekface/mgorus"
)

// CreateLogger sets up a logger instance with configed hooks, etc
func (engine *Engine) CreateLogger() {
	cfg := engine.Config
	logger := logrus.StandardLogger()

	switch cfg.Logging.Level {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	}

	//use json format by default unless we have a blank "type"
	logrus.SetFormatter(&logrus.JSONFormatter{})

	//add hooks to external services
	switch cfg.Logging.Type {
	case "fs":
		logrus.AddHook(lfshook.NewHook(lfshook.PathMap{
			logrus.DebugLevel: cfg.Logging.FsDebugPath,
			logrus.InfoLevel:  cfg.Logging.FsInfoPath,
			logrus.WarnLevel:  cfg.Logging.FsWarnPath,
			logrus.ErrorLevel: cfg.Logging.FsErrorPath,
			logrus.FatalLevel: cfg.Logging.FsFatalPath,
			logrus.PanicLevel: cfg.Logging.FsPanicPath,
		}, &logrus.JSONFormatter{}))

	case "syslog":
		//func to work around no windows syslog stubs in golang
		addSysLogHook(cfg)

	case "mongodb":
		hook, err := mgorus.NewHooker(cfg.Logging.MongoIPPort, cfg.Logging.MongoDb, cfg.Logging.MongoCollection)
		if err != nil {
			logrus.Panic("Log: Couldn't connect to configured mongodb server")
		}
		logrus.AddHook(hook)
	default:
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	engine.Log = logger
}

// LogDebug logs an info message with fields
func (engine *Engine) LogDebug(code string, fields logrus.Fields, message string) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["code"] = code
	engine.Log.WithFields(fields).Debug(message)
}

// LogInfo logs an info message with fields
func (engine *Engine) LogInfo(code string, fields logrus.Fields, message string) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Info(message)
}

// LogWarn logs an info message with fields
func (engine *Engine) LogWarn(code string, fields logrus.Fields, message string) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Warn(message)
}

// LogError logs an info message with fields
func (engine *Engine) LogError(code string, fields logrus.Fields, message string) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Error(message)
}

// LogFatal logs an info message with fields
func (engine *Engine) LogFatal(code string, fields logrus.Fields, message string) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Fatal(message)
}

// LogPanic logs an info message with fields
func (engine *Engine) LogPanic(code string, fields logrus.Fields, message string) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["logcode"] = code
	engine.Log.WithFields(fields).Panic(message)
}
