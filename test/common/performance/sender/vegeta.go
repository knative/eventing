/*
Copyright 2019 The Knative Authors

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

package sender

import (
    "net"
    "net/http"
    "time"

    vegeta "github.com/tsenart/vegeta/lib"
)

// Since we need to add an interceptor to keep track of timestamps before and after sending events,
// we need to have our own Transport implementation.
// At the same time we still need to use the one implemented in Vegeta, which is optimized to being able to generate
// high loads. But since the function is not exported, we need to add it here in order to use it.
// The below function is mostly copied from https://github.com/tsenart/vegeta/blob/44a49c878dd6f28f04b9b5ce5751490b0dce1e18/lib/attack.go#L80
func vegetaAttackerTransport() *http.Transport {
    dialer := &net.Dialer{
        LocalAddr: &net.TCPAddr{IP: vegeta.DefaultLocalAddr.IP, Zone: vegeta.DefaultLocalAddr.Zone},
        KeepAlive: 30 * time.Second,
    }

    return &http.Transport{
        Proxy:               http.ProxyFromEnvironment,
        Dial:                dialer.Dial,
        TLSClientConfig:     vegeta.DefaultTLSConfig,
        MaxIdleConnsPerHost: vegeta.DefaultConnections,
    }
}
