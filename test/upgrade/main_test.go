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

package upgrade

import (
	"knative.dev/eventing/test"
	"knative.dev/eventing/test/common"
	"os"
	"testing"
)

var setup = common.Setup
var tearDown = common.TearDown
var channelTestRunner common.ChannelTestRunner

func TestMain(m *testing.M) {
	test.InitializeEventingFlags()
	channelTestRunner = common.ChannelTestRunner{
		ChannelFeatureMap: common.ChannelFeatureMap,
		ChannelsToTest:    test.EventingFlags.Channels,
	}
	os.Exit(m.Run())
}
