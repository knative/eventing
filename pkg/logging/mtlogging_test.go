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
	"testing"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

func Test_mtLogger_TenantInfo(t *testing.T) {
	type fields struct {
		SugaredLogger zap.SugaredLogger
		clientID      string
	}
	type args struct {
		msg    string
		fields []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{{
		name:   "blah",
		fields: fields{},
		args:   args{},
	},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			log := logging.FromContext(ctx)
			ctx = ContextWithMTLoggerFromLogger(ctx, log, "clientid")
			logger := MTLoggerFromContext(ctx)
			//			logger.TenantInfof("hi rick")
			logger.Info("hi rick ")
			logger.TenantInfo("hi rick to tenant ")
			logger.TenantInfof("To tenant using TenantInfof")
			logger.Info("hi rick 2")
			// m := MTLogger{
			// 	SugaredLogger: tt.fields.SugaredLogger,
			// 	clientID:      tt.fields.clientID,
			// }
		})
	}
}
