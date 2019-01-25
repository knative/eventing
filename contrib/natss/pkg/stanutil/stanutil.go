/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package stanutil

import (
	"errors"
	"fmt"
	"time"

	stan "github.com/nats-io/go-nats-streaming"
	"go.uber.org/zap"
)

// Connect creates a new NATS-Streaming connection
func Connect(clusterId string, clientId string, natsUrl string, logger *zap.SugaredLogger) (*stan.Conn, error) {
	logger.Infof("Connect(): clusterId: %v; clientId: %v; natssUrl: %v", clusterId, clientId, natsUrl)
	var sc stan.Conn
	var err error
	for i := 0; i < 60; i++ {
		if sc, err = stan.Connect(clusterId, clientId, stan.NatsURL(natsUrl)); err != nil {
			logger.Warnf("Connect(): create new connection failed: %v", err)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		logger.Errorf("Connect(): create new connection failed: %v", err)
		return nil, err
	}
	logger.Infof("Connect(): connection to NATSS established, natsConn=%+v", &sc)
	return &sc, nil
}

// Close must be the last call to close the connection
func Close(sc *stan.Conn, logger *zap.SugaredLogger) (err error) {
	defer func() { // the NATSS client could panic if the underlying connectivity is damaged
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from: %v", r)
			logger.Errorf("Close(): %v", err.Error())
		}
	}()

	if sc == nil {
		err = errors.New("can't close empty stan connection")
		return
	}
	err = (*sc).Close()
	if err != nil {
		logger.Errorf("Can't close connection: %+v", err)
	}
	return
}

// Publish a message to a subject
func Publish(sc *stan.Conn, subj string, msg *[]byte, logger *zap.SugaredLogger) (err error) {
	defer func() { // the NATSS client could panic if the underlying connectivity is damaged
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from: %v", r)
			logger.Errorf("Publish(): %v", err.Error())
		}
	}()

	if sc == nil {
		err = errors.New("cant'publish on empty stan connection")
		return
	}
	err = (*sc).Publish(subj, *msg)
	if err != nil {
		logger.Errorf("Error during publish: %v\n", err)
	}
	return
}
