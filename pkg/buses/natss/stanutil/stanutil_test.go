package stanutil

import (
	"github.com/knative/eventing/pkg/buses"
	"github.com/nats-io/nats-streaming-server/server"
	"go.uber.org/zap"
	"os"
	"testing"
)

const (
	clusterId = "knative-eventing"
	clientId  = "testClient"
	natssUrl  = "nats://localhost:4222"
)

var (
	logger *zap.SugaredLogger
)

func TestMain(m *testing.M) {
	logger = buses.NewBusLoggerFromConfig(buses.NewLoggingConfig())
	defer logger.Sync()

	stanServer, err := startNatss()
	if err != nil {
		panic(err)
	}

	retCode := m.Run()

	stopNatss(stanServer)
	os.Exit(retCode)
}

func TestConnectPublishClose(t *testing.T) {
	// connect
	natssConn, err := Connect(clusterId, clientId, natssUrl, logger)
	if err != nil {
		t.Errorf("Connect failed: %v", err)
	}
	logger.Infof("natssConn: %v", natssConn)

	//publish
	msg := []byte("testMessage")
	err = Publish(natssConn, "testTopic", &msg, logger)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}
}

func startNatss() (*server.StanServer, error) {
	return server.RunServer(clusterId)
}

func stopNatss(server *server.StanServer) {
	server.Shutdown()
}
