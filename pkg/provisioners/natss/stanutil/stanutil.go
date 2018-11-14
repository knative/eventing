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
	"go.uber.org/zap"

	stan "github.com/nats-io/go-nats-streaming"
)

// Connect creates a new NATS-Streaming connection
func Connect(clusterID string, clientID string, natsURL string, logger *zap.SugaredLogger) (*stan.Conn, error) {
	sc, err := stan.Connect(clusterID, clientID, stan.NatsURL(natsURL))
	if err != nil {
		logger.Errorf("Can't connect to: %s ; error: %v; NATS URL: %s", clusterID, err, natsURL)
	}
	return &sc, err
}

// Close must be the last call to close the connection
func Close(sc *stan.Conn, logger *zap.SugaredLogger) (err error) {
	defer func() {
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
	defer func() {
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
