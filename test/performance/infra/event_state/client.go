/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package event_state

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
)

const publishTimeout = 1 * time.Minute

type AggregatorClient struct {
	conn   *grpc.ClientConn
	aggCli EventsRecorderClient
}

func NewAggregatorClient(aggregAddr string) (*AggregatorClient, error) {
	// create a connection to the aggregator
	conn, err := grpc.Dial(aggregAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the aggregator: %v", err)
	}

	aggCli := NewEventsRecorderClient(conn)

	return &AggregatorClient{conn, aggCli}, nil
}

func (ac *AggregatorClient) Publish(rl *EventsRecordList) error {
	return ac.publishWithTimeout(publishTimeout, rl)
}

func (ac *AggregatorClient) publishWithTimeout(timeout time.Duration, rl *EventsRecordList) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := ac.aggCli.RecordEvents(ctx, rl)
	return err
}

func (ac *AggregatorClient) Close() {
	_ = ac.conn.Close()
}
