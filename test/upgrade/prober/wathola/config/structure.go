/*
Copyright 2021 The Knative Authors

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

package config

import (
	"time"
)

// ReceiverTeardownConfig holds config receiver teardown
type ReceiverTeardownConfig struct {
	Duration time.Duration `json:"duration"`
}

// ReceiverProgressConfig holds config receiver progress reporting
type ReceiverProgressConfig struct {
	Duration time.Duration `json:"duration"`
}

// ReceiverErrorConfig holds error reporting config of the receiver
type ReceiverErrorConfig struct {
	UnavailablePeriodToReport time.Duration `json:"unavailable-period-to-report"`
}

// ReceiverConfig hold configuration for receiver
type ReceiverConfig struct {
	Teardown ReceiverTeardownConfig `json:"teardown"`
	Progress ReceiverProgressConfig `json:"progress"`
	Errors   ReceiverErrorConfig    `json:"errors"`
	Port     int                    `json:"port"`
}

// SenderConfig hold configuration for sender
type SenderConfig struct {
	Address  interface{}   `json:"address"`
	Interval time.Duration `json:"interval"`
}

// ForwarderConfig holds configuration for forwarder
type ForwarderConfig struct {
	Target string `json:"target"`
	Port   int    `json:"port"`
}

// ReadinessConfig holds a readiness configuration
type ReadinessConfig struct {
	Enabled bool   `json:"enabled"`
	URI     string `json:"uri"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// Config hold complete configuration
type Config struct {
	Sender        SenderConfig    `json:"sender"`
	Forwarder     ForwarderConfig `json:"forwarder"`
	Receiver      ReceiverConfig  `json:"receiver"`
	Readiness     ReadinessConfig `json:"readiness"`
	LogLevel      string          `json:"log-level"`
	TracingConfig string          `json:"tracing"`
}
