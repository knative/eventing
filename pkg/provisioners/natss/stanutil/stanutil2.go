package stanutil

import (
	"time"

	"go.uber.org/zap"
	stan "github.com/nats-io/go-nats-streaming"
	"sync"
)

var (
	natsConn *stan.Conn  // the gloab NATSS connection
	natsConnMux sync.Mutex
)

func initNatssConn(natsUrl string, clientId string, logger *zap.SugaredLogger) error {
	logger.Infof("initNatssConn(): Natss connection: %+v", natsConn)
	natsConnMux.Lock()
	defer natsConnMux.Unlock()
	if natsConn== nil {
		var err error
		for i := 0; i < 60; i++ {
			if natsConn, err = Connect("knative-nats-streaming", clientId, natsUrl, logger); err != nil {
				logger.Errorf("initNatssConn(): create new connection failed: %+v", err)
				time.Sleep(1 * time.Second)
			} else {
				break
			}
		}
		if err != nil {
			logger.Errorf("initNatssConn(): create new connection failed: %+v", err)
			return err
		}
		logger.Infof("initNatssConn(): connection to NATSS established, natsConn=%+v", natsConn)
	}
	logger.Infof("initNatssConn(): connection to NATSS exists, natsConn=%+v", natsConn)
	return nil
}

func GetNatssConnection(natsUrl string, clientId string, logger *zap.SugaredLogger) (*stan.Conn, error) {
	logger.Infof("Natss url: %v, clientId: %v, connection: %+v", natsUrl, clientId, natsConn)
	if err := initNatssConn(natsUrl, clientId, logger); err != nil {
		return nil, err
	}
	return natsConn, nil
}