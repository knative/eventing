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

package fetcher

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/wavesoftware/go-ensure"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/receiver"
)

// New will create a new fetcher
func New() Fetcher {
	ml := &memoryLogger{
		Execution: &Execution{
			Logs:   make([]LogEntry, 0),
			Report: nil,
		},
	}
	config.Log = newLogger(ml)
	config.ReadIfPresent()
	return &fetcher{
		out: os.Stdout,
		log: ml,
	}
}

// FetchReport will try to fetch report from remote receiver service and dump
// it on the output as JSON.
func (f *fetcher) FetchReport() {
	err := f.fetchReport()
	if err != nil {
		config.Log.Error(err)
	}
	f.ensureStatePrinted()
}

func (f *fetcher) fetchReport() error {
	u, err := url.Parse(config.Instance.Forwarder.Target)
	if err != nil {
		return err
	}
	u.Path = "/report"
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	state := &receiver.Report{
		Thrown: []string{},
	}
	err = json.Unmarshal(body, state)
	if err != nil {
		return err
	}
	f.log.Report = state
	return nil
}

func (f *fetcher) ensureStatePrinted() {
	bytes, err := json.MarshalIndent(f.log.Execution, "", "  ")
	ensure.NoError(err)
	_, err = f.out.Write(bytes)
	ensure.NoError(err)
	_, err = f.out.Write([]byte("\n"))
	ensure.NoError(err)
}

func (m *memoryLogger) Write(p []byte) (int, error) {
	le := LogEntry{}
	err := json.Unmarshal(p, &le)
	if err != nil {
		return 0, err
	}
	m.Logs = append(m.Logs, le)
	return len(p), nil
}

func (m *memoryLogger) Sync() error {
	return nil
}

func newLogger(ws zapcore.WriteSyncer) *zap.SugaredLogger {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		TimeKey:        "dt",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), ws, zap.DebugLevel)
	return zap.New(core).Sugar()
}
