package dispatcher

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	"k8s.io/api/core/v1"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
)

type mockConsumer struct {
	message chan *sarama.ConsumerMessage
}

func (c *mockConsumer) Messages() <-chan *sarama.ConsumerMessage {
	return c.message
}

func (c *mockConsumer) Close() error {
	return nil
}

func (c *mockConsumer) MarkOffset(msg *sarama.ConsumerMessage, metadata string) {
	return
}

type mockSaramaCluster struct {
	// closed closes the message channel so that it doesn't block during the test
	closed bool
	// Handle to the latest created consumer, useful to access underlying message chan
	consumerChannel chan *sarama.ConsumerMessage
	// createErr will return an error when creating a consumer
	createErr bool
}

func (c *mockSaramaCluster) NewConsumer(groupID string, topics []string) (KafkaConsumer, error) {
	if c.createErr {
		return nil, fmt.Errorf("error creating consumer")
	}
	consumer := &mockConsumer{
		message: make(chan *sarama.ConsumerMessage),
	}
	if c.closed {
		close(consumer.message)
	}
	c.consumerChannel = consumer.message
	return consumer, nil
}

func TestDispatcher_UpdateConfig(t *testing.T) {
	testCases := []struct {
		name         string
		oldConfig    *multichannelfanout.Config
		newConfig    *multichannelfanout.Config
		subscribes   []string
		unsubscribes []string
		createErr    string
	}{
		{
			name:      "nil config",
			oldConfig: &multichannelfanout.Config{},
			newConfig: nil,
			createErr: "nil config",
		},
		{
			name:      "same config",
			oldConfig: &multichannelfanout.Config{},
			newConfig: &multichannelfanout.Config{},
		},
		{
			name:      "config with no subscription",
			oldConfig: &multichannelfanout.Config{},
			newConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
					},
				},
			},
		},
		{
			name:      "single channel w/ new subscriptions",
			oldConfig: &multichannelfanout.Config{},
			newConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-1",
									},
									SubscriberURI: "http://test/subscriber",
								},
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-2",
									},
									SubscriberURI: "http://test/subscriber",
								},
							},
						},
					},
				},
			},
			subscribes: []string{"subscription-1", "subscription-2"},
		},
		{
			name: "single channel w/ existing subscriptions",
			oldConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-1",
									},
									SubscriberURI: "http://test/subscriber",
								},
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-2",
									},
									SubscriberURI: "http://test/subscriber",
								}}}}},
			},
			newConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-2",
									},
									SubscriberURI: "http://test/subscriber",
								},
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-3",
									},
									SubscriberURI: "http://test/subscriber",
								},
							},
						},
					},
				},
			},
			subscribes:   []string{"subscription-2", "subscription-3"},
			unsubscribes: []string{"subscription-1"},
		},
		{
			name: "multi channel w/old and new subscriptions",
			oldConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel-1",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-1",
									},
									SubscriberURI: "http://test/subscriber",
								},
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-2",
									},
									SubscriberURI: "http://test/subscriber",
								},
							},
						}}}},
			newConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel-1",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-1",
									},
									SubscriberURI: "http://test/subscriber",
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "test-channel-2",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-3",
									},
									SubscriberURI: "http://test/subscriber",
								},
								{
									Ref: &v1.ObjectReference{
										Name: "subscription-4",
									},
									SubscriberURI: "http://test/subscriber",
								},
							},
						},
					},
				},
			},
			subscribes:   []string{"subscription-1", "subscription-3", "subscription-4"},
			unsubscribes: []string{"subscription-2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running %s", t.Name())
			d := &KafkaDispatcher{
				kafkaCluster:   &mockSaramaCluster{closed: true},
				kafkaConsumers: make(map[buses.ChannelReference]map[subscription]KafkaConsumer),

				logger: zap.NewNop(),
			}
			d.setConfig(&multichannelfanout.Config{})

			// Initialize using oldConfig
			err := d.UpdateConfig(tc.oldConfig)
			if err != nil {
				t.Errorf("unexpected error: %s", err)
			}
			oldSubscribers := make(map[string]bool)
			for _, subMap := range d.kafkaConsumers {
				for sub := range subMap {
					oldSubscribers[sub.Name] = true
				}
			}
			for _, sub := range tc.unsubscribes {
				if ok := oldSubscribers[sub]; !ok {
					t.Errorf("subscription %s was never subscribed", sub)
				}
			}

			// Update with new config
			err = d.UpdateConfig(tc.newConfig)
			if tc.createErr != "" {
				if err == nil {
					t.Errorf("Expected UpdateConfig error: '%v'. Actual nil", tc.createErr)
				} else if err.Error() != tc.createErr {
					t.Errorf("Unexpected UpdateConfig error. Expected '%v'. Actual '%v'", tc.createErr, err)
				}
				return
			} else if err != nil {
				t.Errorf("Unexpected UpdateConfig error. Expected nil. Actual '%v'", err)
			}

			var newSubscribers []string
			for _, subMap := range d.kafkaConsumers {
				for sub := range subMap {
					newSubscribers = append(newSubscribers, sub.Name)
				}
			}

			if diff := cmp.Diff(tc.subscribes, newSubscribers, sortStrings); diff != "" {
				t.Errorf("unexpected subscribers (-want, +got) = %v", diff)
			}

		})
	}
}

func TestFromKafkaMessage(t *testing.T) {
	data := []byte("data")
	kafkaMessage := &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("k1"),
				Value: []byte("v1"),
			},
		},
		Value: data,
	}
	want := &buses.Message{
		Headers: map[string]string{
			"k1": "v1",
		},
		Payload: data,
	}
	got := fromKafkaMessage(kafkaMessage)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected message (-want, +got) = %v", diff)
	}
}

func TestToKafkaMessage(t *testing.T) {
	data := []byte("data")
	channelRef := buses.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}
	msg := &buses.Message{
		Headers: map[string]string{
			"k1": "v1",
		},
		Payload: data,
	}
	want := &sarama.ProducerMessage{
		Topic: "test-ns.test-channel",
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("k1"),
				Value: []byte("v1"),
			},
		},
		Value: sarama.ByteEncoder(data),
	}
	got := toKafkaMessage(channelRef, msg)
	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(sarama.ProducerMessage{})); diff != "" {
		t.Errorf("unexpected message (-want, +got) = %v", diff)
	}
}

type dispatchTestHandler struct {
	t       *testing.T
	payload []byte
	done    chan bool
}

func (h *dispatchTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.t.Error("Failed to read the request body")
	}
	if diff := cmp.Diff(h.payload, body); diff != "" {
		h.t.Errorf("unexpected body (-want, +got) = %v", diff)
	}
	h.done <- true
}

func TestSubscribe(t *testing.T) {
	sc := &mockSaramaCluster{}
	data := []byte("data")
	d := &KafkaDispatcher{
		kafkaCluster:   sc,
		kafkaConsumers: make(map[buses.ChannelReference]map[subscription]KafkaConsumer),
		dispatcher:     buses.NewMessageDispatcher(zap.NewNop().Sugar()),
		logger:         zap.NewNop(),
	}

	testHandler := &dispatchTestHandler{
		t:       t,
		payload: data,
		done:    make(chan bool)}

	server := httptest.NewServer(testHandler)
	defer server.Close()

	channelRef := buses.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	subRef := subscription{
		Name:          "test-sub",
		Namespace:     "test-ns",
		SubscriberURI: server.URL[7:],
	}
	err := d.subscribe(channelRef, subRef)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	defer close(sc.consumerChannel)
	sc.consumerChannel <- &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("k1"),
				Value: []byte("v1"),
			},
		},
		Value: data,
	}

	<-testHandler.done

}

func TestSubscribeError(t *testing.T) {
	sc := &mockSaramaCluster{
		createErr: true,
	}
	d := &KafkaDispatcher{
		kafkaCluster: sc,
		logger:       zap.NewNop(),
	}

	channelRef := buses.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	subRef := subscription{
		Name:      "test-sub",
		Namespace: "test-ns",
	}
	err := d.subscribe(channelRef, subRef)
	if err == nil {
		t.Errorf("expected error want %s, got %s", "error creating consumer", err)
	}
}

func TestUnsubscribeUnknownSub(t *testing.T) {
	sc := &mockSaramaCluster{
		createErr: true,
	}
	d := &KafkaDispatcher{
		kafkaCluster: sc,
		logger:       zap.NewNop(),
	}

	channelRef := buses.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	subRef := subscription{
		Name:      "test-sub",
		Namespace: "test-ns",
	}
	err := d.unsubscribe(channelRef, subRef)
	if err != nil {
		t.Errorf("uexpected error %s", err)
	}
}

func TestKafkaDispatcher_Start(t *testing.T) {
	d := &KafkaDispatcher{}
	err := d.Start(make(chan struct{}))
	if err == nil {
		t.Errorf("expected error want %s, got %s", "message receiver is not set", err)
	}

	d.receiver = buses.NewMessageReceiver(func(channel buses.ChannelReference, message *buses.Message) error {
		return nil
	}, zap.NewNop().Sugar())
	err = d.Start(make(chan struct{}))
	if err == nil {
		t.Errorf("expected error want %s, got %s", "kafkaAsyncProducer is not set", err)
	}
}

var sortStrings = cmpopts.SortSlices(func(x, y string) bool {
	return x < y
})
