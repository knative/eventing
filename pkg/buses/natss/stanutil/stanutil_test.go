package stanutil

import (
	"testing"
	"os"
	"github.com/knative/eventing/pkg/buses"
	"github.com/nats-io/nats-streaming-server/server"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	clusterId = "knative-eventing"
	clientId = "testClient"
    natssUrl = "nats://localhost:4222"
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
	natsConn, err := Connect(clusterId, clientId, natssUrl, logger)
	assert.Nil(t, err)
	logger.Info("natsConn: %v", natsConn)

	//publish
	msg := []byte("testMessage")
	err = Publish(natsConn, "testTopic",&msg,logger)
	assert.Nil(t, err)

	//close
	Close(natsConn, logger)
	assert.Nil(t, err)
}

func startNatss() (*server.StanServer, error) {
	return server.RunServer(clusterId)
}

func stopNatss(server *server.StanServer) {
	server.Shutdown()
}
