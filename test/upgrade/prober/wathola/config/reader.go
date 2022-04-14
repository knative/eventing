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
	"io/ioutil"
	"os"

	"github.com/mitchellh/go-homedir"
	"sigs.k8s.io/yaml"
)

const (
	// LocationEnvVariable holds a environmental variable name.
	LocationEnvVariable = "WATHOLA_CONFIG_FILE_LOCATION"
)

var (
	// LogFatal is used to log a fatal error.
	LogFatal        = Log.Fatal
	defaultLocation = "~/.config/wathola/config.yaml"
)

// ReadIfPresent read a configuration file if it exists
func ReadIfPresent() {
	configFile, err := homedir.Expand(configLocation())
	if err != nil {
		LogFatal(err)
	}
	if fileExists(configFile) {
		Log.Infof("Reading config file: %v", configFile)
		err = Read(configFile)
		if err != nil {
			LogFatal(err)
		}
	} else {
		Log.Infof("Define config file to be taken into account: %v", configFile)
	}
	if err = setLogLevel(); err != nil {
		LogFatal(err)
	}
}

// Read a config file and update configuration object
func Read(configFile string) error {
	bytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(bytes, Instance)
}

func configLocation() string {
	location, set := os.LookupEnv(LocationEnvVariable)
	if !set {
		location = defaultLocation
	}
	return location
}

func setLogLevel() error {
	err := logConfig.Level.UnmarshalText([]byte(Instance.LogLevel))
	if err == nil {
		Instance.LogLevel = logConfig.Level.String()
	} else {
		Instance.LogLevel = Defaults().LogLevel
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
