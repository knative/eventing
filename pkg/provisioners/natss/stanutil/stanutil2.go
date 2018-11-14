package stanutil

import (
	"time"

	"go.uber.org/zap"
	stan "github.com/nats-io/go-nats-streaming"
	"sync"
)

var (
	natsConnMux sync.Mutex
)

func InitNatssConnection(natsUrl string, clientId string, logger *zap.SugaredLogger) (*stan.Conn, error) {
	logger.Infof("InitNatssConnection(): natssUrl: %v; clientId=%v", natsUrl, clientId)
	natsConnMux.Lock()
	defer natsConnMux.Unlock()
	var natsConn *stan.Conn
	var err error
	for i := 0; i < 60; i++ {
		if natsConn, err = Connect("knative-nats-streaming", clientId, natsUrl, logger); err != nil {
			logger.Errorf("InitNatssConnection(): create new connection failed: %v", err)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		logger.Errorf("initNatssConn(): create new connection failed: %v", err)
		return nil, err
	}
	logger.Infof("initNatssConn(): connection to NATSS established, natsConn=%+v", natsConn)
	return natsConn, nil
}
