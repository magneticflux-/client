// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package service

import (
	"errors"

	"github.com/keybase/client/go/logger"
	keybase1 "github.com/keybase/client/go/protocol"
)

type logRegister struct {
	forwarder *logFwd
	queue     *logQueue
	logger    logger.Logger
}

func newLogRegister(fwd *logFwd, logger logger.Logger) *logRegister {
	return &logRegister{
		forwarder: fwd,
		logger:    logger,
	}
}

func (r *logRegister) RegisterLogger(arg keybase1.RegisterLoggerArg, ui *LogUI) error {
	if r.queue != nil {
		return errors.New("logger already registered")
	}

	// create a new log queue and add it to the forwarder
	r.queue = newLogQueue(arg.Name, arg.Level, ui)
	r.forwarder.Add(r.queue)

	return nil
}

func (r *logRegister) UnregisterLogger() {
	if r.queue == nil {
		return
	}
	// remove the log queue from the forwarder
	r.forwarder.Remove(r.queue)
	r.logger.Debug("Unregistered logger: %s", r.queue)
}
