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
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/pelletier/go-toml/v2"
)

var location = "~/.config/wathola/config.toml"
var logFatal = Log.Fatal

// ReadIfPresent read a configuration file if it exists
func ReadIfPresent() {
	configFile, err := homedir.Expand(location)
	if err != nil {
		logFatal(err)
	}
	if fileExists(configFile) {
		Log.Infof("Reading config file: %v", configFile)
		err = Read(configFile)
		if err != nil {
			logFatal(err)
		}
	} else {
		Log.Infof("Define config file to be taken into account: %v", configFile)
	}
	if err = setLogLevel(); err != nil {
		logFatal(err)
	}
}

// Read a config file and update configuration object
func Read(configFile string) error {
	r, err := os.Open(configFile)
	if err != nil {
		return err
	}
	d := toml.NewDecoder(r)
	d.SetStrict(true)
	err = d.Decode(Instance)
	return err
}

func setLogLevel() error {
	err := logConfig.Level.UnmarshalText([]byte(Instance.LogLevel))
	if err == nil {
		Instance.LogLevel = logConfig.Level.String()
	} else {
		Instance.LogLevel = defaultValues().LogLevel
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
