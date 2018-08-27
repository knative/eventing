/*
 * Copyright 2018 The Knative Authors
 *
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
 *
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/pkg/signals"
)

const (
	NumPartitions = "NumPartitions"
)

type provisioner struct {
	client          sarama.Client
	admin           sarama.ClusterAdmin
	monitor         *buses.Monitor
	informerFactory informers.SharedInformerFactory
	namespace       string
	name            string
}

func main() {

	sarama.Logger = log.New(os.Stderr, "[Sarama] ", log.LstdFlags)

	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL := flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.Parse()

	name := os.Getenv("BUS_NAME")
	namespace := os.Getenv("BUS_NAMESPACE")

	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 0 {
		log.Fatalf("Environment variable KAFKA_BROKERS not set")
	}

	conf := sarama.NewConfig()
	conf.Version = sarama.V1_1_0_0
	conf.ClientID = name + "-provisioner"

	clusterAdmin, err := sarama.NewClusterAdmin(brokers, conf)
	if err != nil {
		glog.Fatalf("Error building kafka admin client: %v", err)
	}

	provisioner, err := NewKafkaProvisioner(name, namespace, *masterURL, *kubeconfig, clusterAdmin)
	if err != nil {
		glog.Fatalf("Error building kafka provisioner: %v", err)
	}

	stopCh := signals.SetupSignalHandler()
	provisioner.Start(stopCh)

	<-stopCh
	glog.Flush()
}

func NewKafkaProvisioner(name string, namespace string, masterURL string, kubeconfig string, admin sarama.ClusterAdmin) (*provisioner, error) {

	p := provisioner{
		namespace: namespace,
		name:      name,
		admin:     admin,
	}
	component := fmt.Sprintf("%s-%s", name, buses.Provisioner)
	monitor := buses.NewMonitor(component, masterURL, kubeconfig, buses.MonitorEventHandlerFuncs{
		ProvisionFunc:   p.provision,
		UnprovisionFunc: p.unprovision,
	})
	p.monitor = monitor

	return &p, nil
}

func (p *provisioner) Start(stopCh <-chan struct{}) {
	go p.monitor.Run(p.namespace, p.name, 2, stopCh)
}

func (p *provisioner) provision(channel *channelsv1alpha1.Channel, parameters buses.ResolvedParameters) error {
	topicName := topicNameFromChannel(channel)
	glog.Infof("Provisioning topic %s on bus backed by Kafka", topicName)

	partitions := 1
	if p, ok := parameters[NumPartitions]; ok {
		var err error
		partitions, err = strconv.Atoi(p)
		if err != nil {
			glog.Warningf("Could not parse partition count for %s/%s: %s", channel.Namespace, channel.Name, p)
		}
	}

	err := p.admin.CreateTopic(topicName, &sarama.TopicDetail{ReplicationFactor: 1, NumPartitions: int32(partitions)}, false)
	if err == sarama.ErrTopicAlreadyExists {
		return nil
	} else if err != nil {
		glog.Errorf("Error creating topic %s: %v", topicName, err)
	} else {
		glog.Infof("Successfully created topic %s", topicName)
	}
	return err
}

func (p *provisioner) unprovision(channel *channelsv1alpha1.Channel) error {
	topicName := topicNameFromChannel(channel)
	glog.Infof("Un-provisioning topic %s from bus backed by Kafka", topicName)

	err := p.admin.DeleteTopic(topicName)
	if err == sarama.ErrUnknownTopicOrPartition {
		return nil
	} else if err != nil {
		glog.Errorf("Error deleting topic %s: %v", topicName, err)
	} else {
		glog.Infof("Successfully deleted topic %s", topicName)
	}

	return err
}

func topicNameFromChannel(channel *channelsv1alpha1.Channel) string {
	return fmt.Sprintf("%s.%s", channel.Namespace, channel.Name)
}
