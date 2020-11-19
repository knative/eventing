/*
 * Copyright 2020 The Knative Authors
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

package config

import (
	"time"

	"go.uber.org/zap/zapcore"
)

// ReceiverTeardownConfig holds config receiver teardown
type ReceiverTeardownConfig struct {
	Duration time.Duration
}

// ReceiverProgressConfig holds config receiver progress reporting
type ReceiverProgressConfig struct {
	Duration time.Duration
}

// ReceiverConfig hold configuration for receiver
type ReceiverConfig struct {
	Teardown ReceiverTeardownConfig
	Progress ReceiverProgressConfig
	Port     int
}

// SenderConfig hold configuration for sender
type SenderConfig struct {
	Address  string
	Interval time.Duration
	Cooldown time.Duration
}

// ForwarderConfig holds configuration for forwarder
type ForwarderConfig struct {
	Target string
	Port   int
}

// ReadinessConfig holds a readiness configuration
type ReadinessConfig struct {
	Enabled bool
	URI     string
	Message string
	Status  int
}

// Config hold complete configuration
type Config struct {
	Sender    SenderConfig
	Forwarder ForwarderConfig
	Receiver  ReceiverConfig
	Readiness ReadinessConfig
	LogLevel  zapcore.Level
}
