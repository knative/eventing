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
	"github.com/mitchellh/go-homedir"
	"github.com/pelletier/go-toml"
	"github.com/wavesoftware/go-ensure"

	"os"
)

var location = "~/.config/wathola/config.toml"
var logFatal = Log.Fatal

// ReadIfPresent read a configuration file if it exists
func ReadIfPresent() {
	configFile, err := homedir.Expand(location)
	ensure.NoError(err)
	if fileExists(configFile) {
		Log.Infof("Reading config file: %v", configFile)
		err := Read(configFile)
		if err != nil {
			logFatal(err)
		}
	} else {
		Log.Infof("Define config file to be taken into account: %v", configFile)
	}
}

// Read a config file and update configuration object
func Read(configFile string) error {
	r, err := os.Open(configFile)
	if err != nil {
		return err
	}
	d := toml.NewDecoder(r)
	err = d.Decode(Instance)
	if err == nil {
		logConfig.Level.SetLevel(Instance.LogLevel)
	}
	return err
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
