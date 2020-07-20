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
package logging

import (
	"context"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

type MTLogger struct {
	*zap.SugaredLogger                    // Logger for system
	tenantLogger       *zap.SugaredLogger //Logger providing the tenant key
	clientID           string             //client id usually the namespace
}

const tenantLoggingKey = "tenantNamespace"

func (m MTLogger) TenantInfof(template string, args ...interface{}) {
	m.tenantLogger.Infof(template, args...)
}

func (m MTLogger) TenantInfo(args ...interface{}) {
	m.tenantLogger.Info(args...)
}

func (m MTLogger) TenantError(args ...interface{}) {
	m.tenantLogger.Info(args...)
}

func (m MTLogger) TenantErrorf(template string, args ...interface{}) {
	m.tenantLogger.Errorf(template, args...)
}

func (m MTLogger) TenantDebug(args ...interface{}) {
	m.tenantLogger.Debug(args...)
}

func (m MTLogger) TenantDebugf(template string, args ...interface{}) {
	m.tenantLogger.Debugf(template, args...)
}
func (m MTLogger) TenantWarn(args ...interface{}) {
	m.tenantLogger.Warn(args...)
}

func (m MTLogger) TenantWarnf(template string, args ...interface{}) {
	m.tenantLogger.Warnf(template, args...)
}

var contextkeyMT struct{}

func ContextWithMTLoggerFromLogger(ctx context.Context, log *zap.SugaredLogger, clientID string) context.Context {

	if log == nil {
		return ctx // should not happen
	}

	if mtLogger, ok := ctx.Value(contextkeyMT).(*MTLogger); ok {
		if mtLogger.clientID == clientID {
			return ctx
		}
	}
	return context.WithValue(ctx, contextkeyMT, &MTLogger{
		SugaredLogger: log,
		tenantLogger:  createTenantLogger(log, clientID),
		clientID:      clientID,
	})
}

func MTLoggerFromContext(ctx context.Context) *MTLogger {

	mtLogger, ok := ctx.Value(contextkeyMT).(*MTLogger)
	if !ok {
		log := logging.FromContext(ctx)
		mtLogger = &MTLogger{
			SugaredLogger: log,
			tenantLogger:  createTenantLogger(log, ""),
		}
		context.WithValue(ctx, contextkeyMT, mtLogger)
	}
	return mtLogger
}

func createTenantLogger(log *zap.SugaredLogger, clientID string) *zap.SugaredLogger {
	tenantLogger := log
	if clientID != "" {
		tenantLogger = log.With(zap.String(tenantLoggingKey, clientID))
	}
	desugerLogger := tenantLogger.Desugar()
	desugerLogger = desugerLogger.WithOptions(zap.AddCallerSkip(1)) // skip 1 stackframe on report.
	return desugerLogger.Sugar()

}
