/*
Copyright 2025 The Knative Authors

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

package requestreply

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1informers "knative.dev/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	eventingv1alpha1listers "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 1000
)

type IngressHandler struct {
	dispatcher         *kncloudevents.Dispatcher
	logger             *zap.Logger
	requestReplyLister eventingv1alpha1listers.RequestReplyLister
	keyStore           AESKeyStore

	requestLock sync.RWMutex
	entries     map[types.NamespacedName]map[string]*proxiedRequest
}

type proxiedRequest struct {
	received       time.Time
	responseWriter http.ResponseWriter
	replyEvent     chan *cloudevents.Event
}

func NewHandler(logger *zap.Logger, requestReplyInformer eventingv1alpha1informers.RequestReplyInformer, trustBundleConfigMapLister corev1listers.ConfigMapNamespaceLister, keyStore AESKeyStore) *IngressHandler {
	connectionArgs := kncloudevents.ConnectionArgs {
		MaxIdleConns: defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}

	kncloudevents.ConfigureConnectionArgs(&connectionArgs)

	clientConfig := eventingtls.ClientConfig {
		TrustBundleConfigMapLister: trustBundleConfigMapLister,
	}

	return &IngressHandler{
		logger: logger,
		dispatcher: kncloudevents.NewDispatcher(clientConfig, nil),
		requestReplyLister: requestReplyInformer.Lister(),
		keyStore: keyStore,

		entries: make(map[types.NamespacedName]map[string]*proxiedRequest),
	}

}

func (h *IngressHandler) HandleRequest(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Allow", "POST, OPTIONS")
	// validate request method
	if req.Method == http.MethodOptions {
		w.Header().Set("WebHook-Allowed-Origin", "*") // Accept from any Origin
		w.Header().Set("WebHook-Allowed-Rate", "*")   // Unlimited requests/minute
		w.WriteHeader(http.StatusOK)
		return
	}
	if req.Method != http.MethodPost {
		h.logger.Warn("unexpected request method", zap.String("method", req.Method))
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// validate request URI
	if req.RequestURI == "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	nsRequestReplyName := strings.Split(strings.TrimSuffix(req.RequestURI, "/"), "/")
	if len(nsRequestReplyName) != 3 {
		h.logger.Info("Malformed uri", zap.String("uri", req.RequestURI))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// extract event from request
	message := cehttp.NewMessageFromHttpRequest(req)
	defer message.Finish(nil)

	ctx := context.Background()

	event, err := binding.ToEvent(ctx, message)
	if err != nil {
		h.logger.Warn("failed to extract event from request", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// validate the extracted event
	if validationErr := event.Validate(); validationErr != nil {
		h.logger.Warn("failed to validate extracted event", zap.Error(validationErr))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	requestReplyNs, requestReplyName := nsRequestReplyName[1], nsRequestReplyName[2]
	requestReply, err := h.getRequestReply(requestReplyName, requestReplyNs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute) // TODO: make this timeout configureable
	defer cancel()

	if isResponseEvent(event) {
		h.handleResponseEvent(ctx, w, event, requestReply)
	} else {
		h.handleNewEvent(ctx, w, event, requestReply, utils.PassThroughHeaders(req.Header))
	}
}

func (h *IngressHandler) getRequestReply(name, namespace string) (*v1alpha1.RequestReply, error) {
	rr, err := h.requestReplyLister.RequestReplies(namespace).Get(name)
	if err != nil {
		h.logger.Warn("RequestReply getter failed")
		return nil, err
	}
	return rr, nil
}

func (h *IngressHandler) getBrokerAddress(rr *v1alpha1.RequestReply) (*duckv1.Addressable, error) {
	if rr.Status.Annotations == nil {
		return nil, fmt.Errorf("RequestReply status annotations uninitialized")
	}

	address, ok := rr.Status.Annotations[v1alpha1.RequestReplyBrokerAddressStatusAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("broker address not found in RequestReply status annotations")
	}

	url, err := apis.ParseURL(address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse broker address url")
	}

	addr := &duckv1.Addressable{
		URL: url,
	}

	if certs, ok := rr.Status.Annotations[v1alpha1.RequestReplyBrokerCACertsStatusAnnotationKey]; ok && certs != "" {
		addr.CACerts = &certs
	}

	if audience, ok := rr.Status.Annotations[v1alpha1.RequestReplyBrokerAudienceStatusAnnotationKey]; ok && audience != "" {
		addr.Audience = &audience
	}

	return addr, nil
}

func (h *IngressHandler) addEvent(responseWriter http.ResponseWriter, event *cloudevents.Event, rr *v1alpha1.RequestReply) *proxiedRequest {
	h.requestLock.Lock()
	defer h.requestLock.Unlock()

	id := event.ID()
	pr := &proxiedRequest{
		received:       time.Now(),
		responseWriter: responseWriter,
		replyEvent:     make(chan *cloudevents.Event, 1),
	}
	h.entries[rr.GetNamespacedName()][id] = pr

	return pr
}

func (h *IngressHandler) deleteEvent(event *cloudevents.Event, rr *v1alpha1.RequestReply) {
	h.requestLock.Lock()
	defer h.requestLock.Unlock()
	delete(h.entries[rr.GetNamespacedName()], event.ID())
}

func (h *IngressHandler) handleNewEvent(ctx context.Context, responseWriter http.ResponseWriter, event *cloudevents.Event, rr *v1alpha1.RequestReply, headers http.Header) {
	h.logger.Debug("handling new event")
	pr := h.addEvent(responseWriter, event, rr)

	brokerAddress, err := h.getBrokerAddress(rr)
	if err != nil {

	}

	latestKey, ok := h.keyStore.GetLatestKey(rr.GetNamespacedName())
	if !ok {
		h.logger.Warn("no aes key found for requestreply resource", zap.String("name", rr.GetName()), zap.String("namespace", rr.GetNamespace()))
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = SetCorrelationId(event, rr.Spec.CorrelationAttribute, latestKey)
	if err != nil {
		h.logger.Error("failed to set correlation id on event", zap.Error(err))
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	opts := []kncloudevents.SendOption{
		kncloudevents.WithHeader(headers),
		// TODO: add oidc stuff here
	}

	_, err = h.dispatcher.SendEvent(ctx, *event, *brokerAddress, opts...)
	if err != nil {
		h.logger.Error("failed to dispatch event", zap.Error(err))
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	for {
		select {
		case resp := <-pr.replyEvent:
			msg := binding.ToMessage(resp)
			err := cehttp.WriteResponseWriter(ctx, msg, http.StatusOK, pr.responseWriter)
			if err != nil {
				h.logger.Error("failed to send event back", zap.Error(err))
			}
			msg.Finish(err)
			h.deleteEvent(event, rr)
			return
		case <-ctx.Done():
			h.logger.Warn("context timeout reached before encountering a response to the event, discarding event", zap.String("event id", event.ID()))
			h.deleteEvent(event, rr)
			return
		}
	}
}
func (h *IngressHandler) handleResponseEvent(ctx context.Context, responseWriter http.ResponseWriter, event *cloudevents.Event, rr *v1alpha1.RequestReply) {
	h.requestLock.RLock()
	defer h.requestLock.RUnlock()

	h.logger.Debug("handling a response event")

	// TODO: with OIDC enabled, we can skip validation of the key if we validate the identity of the trigger making the request
	// This can be handled in a future PR to add the OIDC support for the RequestReply resource
	allKeys, ok := h.keyStore.GetAllKeys(rr.GetNamespacedName())
	if !ok {
		h.logger.Warn("no aes keys found for requestreply resource", zap.String("name", rr.GetName()), zap.String("namespace", rr.GetNamespace()))
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	replyId, ok := event.Extensions()[rr.Spec.ReplyAttribute]
	if !ok {
		h.logger.Warn("no reply id set on the event")
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}

	replyIdString, ok := replyId.(string)
	if !ok {
		h.logger.Warn("failed to convert the replyid to a string")
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}

	validReply := false
	for _, key := range allKeys {
		valid, err := VerifyReplyId(replyIdString, key)
		if err != nil {
			h.logger.Warn("ran into an error validating replyid", zap.Error(err))
			continue
		}
		if valid {
			validReply = true
			break
		}
	}

	if !validReply {
		h.logger.Warn("received invalid reply event")
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}

	responseWriter.WriteHeader(http.StatusAccepted)

	id := strings.Split(replyIdString, ":")[0]
	pr, ok := h.entries[rr.GetNamespacedName()][id]
	if !ok {
		h.logger.Warn("no event found matching the reply id, discarding event", zap.String("reply id", id))
		return
	}

	// send the reply event back to the original response writer
	pr.replyEvent <- event
}

type requestReplyAesKeyStore struct {
	lock      sync.RWMutex
	newestKey []byte
	entries   map[string][]byte
}

func (ks *requestReplyAesKeyStore) addAesKey(keyName string, keyValue []byte) {
	ks.lock.Lock()
	defer ks.lock.Unlock()

	ks.newestKey = keyValue

	if ks.entries == nil {
		ks.entries = map[string][]byte{keyName: keyValue}
		return
	}

	ks.entries[keyName] = keyValue
}

func (ks *requestReplyAesKeyStore) removeAesKey(keyName string) {
	ks.lock.Lock()
	defer ks.lock.Unlock()

	delete(ks.entries, keyName)
}

func (ks *requestReplyAesKeyStore) getLatestKey() []byte {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	return ks.newestKey
}

func (ks *requestReplyAesKeyStore) getAllKeys() [][]byte {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	keys := make([][]byte, len(ks.entries))

	i := 0
	for _, v := range ks.entries {
		keys[i] = v
		i++
	}

	return keys
}

type AESKeyStore struct {
	keyStores map[types.NamespacedName]*requestReplyAesKeyStore
}

func (ks *AESKeyStore) AddAesKey(rrName types.NamespacedName, keyName string, keyValue []byte) {
	if ks.keyStores[rrName] == nil {
		ks.keyStores[rrName] = &requestReplyAesKeyStore{}
	}

	ks.keyStores[rrName].addAesKey(keyName, keyValue)
}

func (ks *AESKeyStore) RemoveAesKey(rrName types.NamespacedName, keyName string) {
	if ks.keyStores[rrName] == nil {
		return
	}

	ks.keyStores[rrName].removeAesKey(keyName)
}

func (ks *AESKeyStore) GetLatestKey(rrName types.NamespacedName) ([]byte, bool) {
	if ks.keyStores[rrName] == nil {
		return nil, false
	}

	return ks.keyStores[rrName].getLatestKey(), true
}

func (ks *AESKeyStore) GetAllKeys(rrName types.NamespacedName) ([][]byte, bool) {
	if ks.keyStores[rrName] == nil {
		return nil, false
	}

	return ks.keyStores[rrName].getAllKeys(), true
}

func isResponseEvent(event *cloudevents.Event) bool {
	_, ok := event.Extensions()["responseid"] // TODO: refactor to use id name from the rr resource
	return ok
}
