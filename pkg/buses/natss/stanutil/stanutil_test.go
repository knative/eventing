package stanutil

import (
	"github.com/knative/eventing/pkg/buses"
	"github.com/nats-io/nats-streaming-server/server"
	"go.uber.org/zap"
	"os"
	"testing"
	"time"
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
	defer stopNatss(stanServer)

	retCode := m.Run()

	os.Exit(retCode)
}

func TestConnectPublishClose(t *testing.T) {
	// connect
	natssConn, err := Connect(clusterId, clientId, natssUrl, logger)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer Close(natssConn, logger)
	logger.Infof("natssConn: %v", natssConn)

	//publish
	msg := []byte("testMessage")
	err = Publish(natssConn, "testTopic", &msg, logger)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}
}

func startNatss() (*server.StanServer, error) {
	var err error
	var stanServer *server.StanServer
	for i := 0; i < 10; i++ {
		if stanServer, err = server.RunServer(clusterId); err != nil {
			logger.Errorf("Start NATSS failed: %+v", err)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	return stanServer, nil
}

func stopNatss(server *server.StanServer) {
	server.Shutdown()
}
