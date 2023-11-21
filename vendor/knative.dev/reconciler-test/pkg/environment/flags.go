/*
Copyright 2020 The Knative Authors

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

package environment

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"
)

var (
	s = new(feature.States)
	l = new(feature.Levels)
	f = new(string)

	testNamespace = new(string)

	ipFilePath     = new(string)
	teardownOnFail = new(bool)

	pollTimeout  = new(time.Duration)
	pollInterval = new(time.Duration)
)

// InitFlags registers the requirement and state filter flags supported by the
// testing framework.
func InitFlags(fs *flag.FlagSet) {
	// States
	*s = feature.Any
	for state, name := range feature.StatesMapping {
		flagName := "feature." + strings.ReplaceAll(strings.ToLower(name), " ", "")
		usage := fmt.Sprintf("toggles %q state assertions", name)
		fs.Var(stateValue{state, s}, flagName, usage)
	}
	fs.Var(stateValue{feature.Any, s}, "feature.any", "toggles all features")

	// Levels
	*l = feature.All
	for level, name := range feature.LevelMapping {
		flagName := "requirement." + strings.ReplaceAll(strings.ToLower(name), " ", "")
		usage := fmt.Sprintf("toggles %q requirement assertions", name)
		fs.Var(levelValue{level, l}, flagName, usage)
	}
	fs.Var(levelValue{feature.All, l}, "requirement.all", "toggles all requirement assertions")

	// Feature
	fs.StringVar(f, "feature", "", "run only Features matching `regexp`")

	fs.StringVar(ipFilePath, "images.producer.file", "", "file path for file-based image producer")
	fs.StringVar(testNamespace, "environment.namespace", "", "Test namespace")
	fs.DurationVar(pollTimeout, "poll.timeout", state.DefaultPollTimeout, "Poll timeout")
	fs.DurationVar(pollInterval, "poll.interval", state.DefaultPollInterval, "Poll interval")
	fs.BoolVar(teardownOnFail, "teardown.on.fail", false, "Set this flag to do teardown even if test fails.")
}

type stateValue struct {
	mask  feature.States
	value *feature.States
}

func (sv stateValue) Get() interface{} {
	return *sv.value & sv.mask
}

func (sv stateValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}

	if v {
		*sv.value = *sv.value | sv.mask // set
	} else {
		*sv.value = *sv.value &^ sv.mask // clear
	}

	return nil
}

func (sv stateValue) IsBoolFlag() bool {
	return true
}

func (sv stateValue) String() string {
	if sv.value != nil && sv.mask&*sv.value != 0 {
		return "true"
	}

	return "false"
}

type levelValue struct {
	mask  feature.Levels
	value *feature.Levels
}

func (lv levelValue) Get() interface{} {
	return *lv.value & lv.mask
}

func (lv levelValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}

	if v {
		*lv.value = *lv.value | lv.mask // set
	} else {
		*lv.value = *lv.value &^ lv.mask // clear
	}

	fmt.Println(lv.value.String(), " set level to ", s)

	return nil
}

func (lv levelValue) IsBoolFlag() bool {
	return true
}

func (lv levelValue) String() string {
	if lv.value != nil && lv.mask&*lv.value != 0 {
		return "true"
	}

	return "false"
}
