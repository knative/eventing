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
	"github.com/golang/glog"

	stan "github.com/nats-io/go-nats-streaming"
)

// Connect creates a new NATS-Streaming connection
func Connect(clusterID string, clientID string, natsURL string) (*stan.Conn, error) {
	sc, err := stan.Connect(clusterID, clientID, stan.NatsURL(natsURL))
	if err != nil {
		glog.Errorf("Can't connect to: %s ; error: %v; NATS URL: %s", clusterID, err, natsURL)
	}
	return &sc, err
}

// Close must be the last call to close the connection
func Close(sc *stan.Conn) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from: %v", r)
			glog.Errorf("Close(): %v\n", err.Error())
		}
	}()

	if sc == nil {
		err = errors.New("can't close empty stan connection")
		return
	}
	err = (*sc).Close()
	if err != nil {
		glog.Errorf("Can't close connection: %+v\n", err)
	}
	return
}

// Publish a message to a subject
func Publish(sc *stan.Conn, subj string, msg *[]byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from: %v", r)
			glog.Errorf("Publish(): %v\n", err.Error())
		}
	}()

	if sc == nil {
		err = errors.New("cant'publish on empty stan connection")
		return
	}
	err = (*sc).Publish(subj, *msg)
	if err != nil {
		glog.Errorf("Error during publish: %v\n", err)
	}
	return
}

// IsConnected ....
func IsConnected(sc *stan.Conn) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
			glog.Errorf("IsConnected() recovered: %v\n", r)
		}
	}()

	if sc != nil && sc != (*stan.Conn)(nil) && (*sc).NatsConn() != nil {
		ok = (*sc).NatsConn().IsConnected()
	} else {
		ok = false
	}
	return
}


