/*
 Copyright 2023 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package mqtt_paho

import (
	"fmt"

	"github.com/eclipse/paho.golang/paho"
)

// Option is the function signature required to be considered an mqtt_paho.Option.
type Option func(*Protocol) error

// WithConnect sets the paho.Connect configuration for the client. This option is not required.
func WithConnect(connOpt *paho.Connect) Option {
	return func(p *Protocol) error {
		if connOpt == nil {
			return fmt.Errorf("the paho.Connect option must not be nil")
		}
		p.connOption = connOpt
		return nil
	}
}

// WithPublish sets the paho.Publish configuration for the client. This option is required if you want to send messages.
func WithPublish(publishOpt *paho.Publish) Option {
	return func(p *Protocol) error {
		if publishOpt == nil {
			return fmt.Errorf("the paho.Publish option must not be nil")
		}
		p.publishOption = publishOpt
		return nil
	}
}

// WithSubscribe sets the paho.Subscribe configuration for the client. This option is required if you want to receive messages.
func WithSubscribe(subscribeOpt *paho.Subscribe) Option {
	return func(p *Protocol) error {
		if subscribeOpt == nil {
			return fmt.Errorf("the paho.Subscribe option must not be nil")
		}
		p.subscribeOption = subscribeOpt
		return nil
	}
}
