package dispatcher

import (
	"errors"
	"fmt"
	"github.com/bsm/sarama-cluster"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	_ "github.com/knative/pkg/system/testing"
)

type mockConsumer struct {
	message    chan *sarama.ConsumerMessage
	partitions chan cluster.PartitionConsumer
}

func (c *mockConsumer) Messages() <-chan *sarama.ConsumerMessage {
	return c.message
}

func (c *mockConsumer) Partitions() <-chan cluster.PartitionConsumer {
	return c.partitions
}

func (c *mockConsumer) Close() error {
	return nil
}

func (c *mockConsumer) MarkOffset(msg *sarama.ConsumerMessage, metadata string) {
	return
}

type mockPartitionConsumer struct {
	highWaterMarkOffset int64
	l                   sync.Mutex
	topic               string
	partition           int32
	offset              int64
	messages            chan *sarama.ConsumerMessage
	errors              chan *sarama.ConsumerError
	singleClose         sync.Once
	consumed            bool
}

// AsyncClose implements the AsyncClose method from the sarama.PartitionConsumer interface.
func (pc *mockPartitionConsumer) AsyncClose() {
	pc.singleClose.Do(func() {
		close(pc.messages)
		close(pc.errors)
	})
}

// Close implements the Close method from the sarama.PartitionConsumer interface. It will
// verify whether the partition consumer was actually started.
func (pc *mockPartitionConsumer) Close() error {
	if !pc.consumed {
		return fmt.Errorf("Expectations set on %s/%d, but no partition consumer was started.", pc.topic, pc.partition)
	}
	pc.AsyncClose()
	var (
		closeErr error
		wg       sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var errs = make(sarama.ConsumerErrors, 0)
		for err := range pc.errors {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			closeErr = errs
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range pc.messages {
			// drain
		}
	}()
	wg.Wait()
	return closeErr
}

// Errors implements the Errors method from the sarama.PartitionConsumer interface.
func (pc *mockPartitionConsumer) Errors() <-chan *sarama.ConsumerError {
	return pc.errors
}

// Messages implements the Messages method from the sarama.PartitionConsumer interface.
func (pc *mockPartitionConsumer) Messages() <-chan *sarama.ConsumerMessage {
	return pc.messages
}

func (pc *mockPartitionConsumer) HighWaterMarkOffset() int64 {
	return atomic.LoadInt64(&pc.highWaterMarkOffset) + 1
}

func (pc *mockPartitionConsumer) Topic() string {
	return pc.topic
}

// Partition returns the consumed partition
func (pc *mockPartitionConsumer) Partition() int32 {
	return pc.partition
}

// InitialOffset returns the offset used for creating the PartitionConsumer instance.
// The returned offset can be a literal offset, or OffsetNewest, or OffsetOldest
func (pc *mockPartitionConsumer) InitialOffset() int64 {
	return 0
}

// MarkOffset marks the offset of a message as preocessed.
func (pc *mockPartitionConsumer) MarkOffset(offset int64, metadata string) {
}

// ResetOffset resets the offset to a previously processed message.
func (pc *mockPartitionConsumer) ResetOffset(offset int64, metadata string) {
	pc.offset = 0
}

type mockSaramaCluster struct {
	// closed closes the message channel so that it doesn't block during the test
	closed bool
	// Handle to the latest created consumer, useful to access underlying message chan
	consumerChannel chan *sarama.ConsumerMessage
	// Handle to the latest created partition consumer, useful to access underlying message chan
	partitionConsumerChannel chan cluster.PartitionConsumer
	// createErr will return an error when creating a consumer
	createErr bool
	// consumer mode
	consumerMode cluster.ConsumerMode
}

func (c *mockSaramaCluster) NewConsumer(groupID string, topics []string) (KafkaConsumer, error) {
	if c.createErr {
		return nil, errors.New("error creating consumer")
	}

	var consumer *mockConsumer
	if c.consumerMode != cluster.ConsumerModePartitions {
		consumer = &mockConsumer{
			message: make(chan *sarama.ConsumerMessage),
		}
		if c.closed {
			close(consumer.message)
		}
		c.consumerChannel = consumer.message
	} else {
		consumer = &mockConsumer{
			partitions: make(chan cluster.PartitionConsumer),
		}
		if c.closed {
			close(consumer.partitions)
		}
		c.partitionConsumerChannel = consumer.partitions
	}
	return consumer, nil
}

func (c *mockSaramaCluster) GetConsumerMode() cluster.ConsumerMode {
	return c.consumerMode
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
				kafkaConsumers: make(map[provisioners.ChannelReference]map[subscription]KafkaConsumer),

				logger: zap.NewNop(),
			}
			d.setConfig(&multichannelfanout.Config{})

			// Initialize using oldConfig
			err := d.UpdateConfig(tc.oldConfig)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			oldSubscribers := sets.NewString()
			for _, subMap := range d.kafkaConsumers {
				for sub := range subMap {
					oldSubscribers.Insert(sub.Name)
				}
			}
			if diff := sets.NewString(tc.unsubscribes...).Difference(oldSubscribers); diff.Len() != 0 {
				t.Errorf("subscriptions %+v were never subscribed", diff)
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
	want := &provisioners.Message{
		Headers: map[string]string{
			"k1": "v1",
		},
		Payload: data,
	}
	got := fromKafkaMessage(kafkaMessage)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected message (-want, +got) = %s", diff)
	}
}

func TestToKafkaMessage(t *testing.T) {
	data := []byte("data")
	channelRef := provisioners.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}
	msg := &provisioners.Message{
		Headers: map[string]string{
			"k1": "v1",
		},
		Payload: data,
	}
	want := &sarama.ProducerMessage{
		Topic: "knative-eventing-channel.test-ns.test-channel",
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
		t.Errorf("unexpected message (-want, +got) = %s", diff)
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
		h.t.Errorf("unexpected body (-want, +got) = %s", diff)
	}
	h.done <- true
}

func TestSubscribe(t *testing.T) {
	sc := &mockSaramaCluster{}
	data := []byte("data")
	d := &KafkaDispatcher{
		kafkaCluster:   sc,
		kafkaConsumers: make(map[provisioners.ChannelReference]map[subscription]KafkaConsumer),
		dispatcher:     provisioners.NewMessageDispatcher(zap.NewNop().Sugar()),
		logger:         zap.NewNop(),
	}

	testHandler := &dispatchTestHandler{
		t:       t,
		payload: data,
		done:    make(chan bool)}

	server := httptest.NewServer(testHandler)
	defer server.Close()

	channelRef := provisioners.ChannelReference{
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

func TestPartitionConsumer(t *testing.T) {
	sc := &mockSaramaCluster{consumerMode: cluster.ConsumerModePartitions}
	data := []byte("data")
	d := &KafkaDispatcher{
		kafkaCluster:   sc,
		kafkaConsumers: make(map[provisioners.ChannelReference]map[subscription]KafkaConsumer),
		dispatcher:     provisioners.NewMessageDispatcher(zap.NewNop().Sugar()),
		logger:         zap.NewNop(),
	}
	testHandler := &dispatchTestHandler{
		t:       t,
		payload: data,
		done:    make(chan bool)}
	server := httptest.NewServer(testHandler)
	defer server.Close()
	channelRef := provisioners.ChannelReference{
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
	defer close(sc.partitionConsumerChannel)
	pc := &mockPartitionConsumer{
		topic:     channelRef.Name,
		partition: 1,
		messages:  make(chan *sarama.ConsumerMessage, 1),
	}
	pc.messages <- &sarama.ConsumerMessage{
		Topic:     channelRef.Name,
		Partition: 1,
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("k1"),
				Value: []byte("v1"),
			},
		},
		Value: data,
	}
	sc.partitionConsumerChannel <- pc
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

	channelRef := provisioners.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	subRef := subscription{
		Name:      "test-sub",
		Namespace: "test-ns",
	}
	err := d.subscribe(channelRef, subRef)
	if err == nil {
		t.Errorf("Expected error want %s, got %s", "error creating consumer", err)
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

	channelRef := provisioners.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	subRef := subscription{
		Name:      "test-sub",
		Namespace: "test-ns",
	}
	if err := d.unsubscribe(channelRef, subRef); err != nil {
		t.Errorf("Unsubscribe error: %v", err)
	}
}

func TestKafkaDispatcher_Start(t *testing.T) {
	d := &KafkaDispatcher{}
	err := d.Start(make(chan struct{}))
	if err == nil {
		t.Errorf("Expected error want %s, got %s", "message receiver is not set", err)
	}

	d.receiver = provisioners.NewMessageReceiver(func(channel provisioners.ChannelReference, message *provisioners.Message) error {
		return nil
	}, zap.NewNop().Sugar())
	err = d.Start(make(chan struct{}))
	if err == nil {
		t.Errorf("Expected error want %s, got %s", "kafkaAsyncProducer is not set", err)
	}
}

var sortStrings = cmpopts.SortSlices(func(x, y string) bool {
	return x < y
})
