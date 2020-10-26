package logger_vent

import "knative.dev/eventing/test/lib/recordevents"

type Logger func(string, ...interface{})

func (l Logger) Vent(observed recordevents.EventInfo) error {
	l("%s", observed)

	return nil
}
