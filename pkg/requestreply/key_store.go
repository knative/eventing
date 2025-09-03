package requestreply

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
)

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
	lock      sync.RWMutex
	keyStores map[types.NamespacedName]*requestReplyAesKeyStore
	done      chan bool
	Logger    *zap.SugaredLogger
}

func (ks *AESKeyStore) AddAesKey(rrName types.NamespacedName, keyName string, keyValue []byte) {
	ks.lock.Lock()
	defer ks.lock.Unlock()

	if ks.keyStores == nil {
		ks.keyStores = make(map[types.NamespacedName]*requestReplyAesKeyStore)
	}

	if ks.keyStores[rrName] == nil {
		ks.keyStores[rrName] = &requestReplyAesKeyStore{}
	}

	ks.keyStores[rrName].addAesKey(keyName, keyValue)
}

func (ks *AESKeyStore) RemoveAesKey(rrName types.NamespacedName, keyName string) {
	ks.lock.Lock()
	defer ks.lock.Unlock()

	if ks.keyStores[rrName] == nil {
		return
	}

	ks.keyStores[rrName].removeAesKey(keyName)
}

func (ks *AESKeyStore) GetLatestKey(rrName types.NamespacedName) ([]byte, bool) {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	if ks.keyStores[rrName] == nil {
		return nil, false
	}

	return ks.keyStores[rrName].getLatestKey(), true
}

func (ks *AESKeyStore) GetAllKeys(rrName types.NamespacedName) ([][]byte, bool) {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	if ks.keyStores[rrName] == nil {
		return nil, false
	}

	return ks.keyStores[rrName].getAllKeys(), true
}

func (ks *AESKeyStore) StopWatch() {
	ks.done <- true
	close(ks.done)
}

func (ks *AESKeyStore) WatchPath(basePath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	if ks.done == nil {
		ks.done = make(chan bool)
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				ks.Logger.Debug("received fsnotify event", zap.Any("event", event))
				if !ok {
					return
				}
				ks.handleEvent(event, watcher)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				ks.Logger.Error("file watcher encountered an error", zap.Error(err))
			case done, ok := <-ks.done:
				if done || !ok {
					ks.Logger.Info("closing watcher...")
					return
				}
			}
		}

	}()

	ks.Logger.Infof("starting watch of files in %s", basePath)
	watcher.Add(basePath)

	ks.Logger.Infof("performing initial scan of files in %s", basePath)
	return filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			ks.processKeyFile(path)
		}

		return nil
	})
}

func (ks *AESKeyStore) handleEvent(event fsnotify.Event, watcher *fsnotify.Watcher) {
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(event.Name)
		if err != nil {
			ks.Logger.Warnw("failed to stat file", zap.String("file", event.Name), zap.Error(err))
			return
		}

		if info.Mode().IsRegular() {
			ks.processKeyFile(event.Name)
		}
	}

	if event.Has(fsnotify.Write) {
		ks.processKeyFile(event.Name)
	}

	if event.Has(fsnotify.Remove) {
		ks.removeKeyFile(event.Name)
	}
}

func (ks *AESKeyStore) processKeyFile(path string) {
	rrNsName, keyName, err := parseFileName(filepath.Base(path))
	if err != nil {
		ks.Logger.Errorw("failed to parse key file path", zap.String("path", path), zap.Error(err))
		return
	}

	keyValue, err := os.ReadFile(path)
	if err != nil {
		ks.Logger.Errorw("failed to read key file", zap.String("path", path), zap.Error(err))
		return
	}

	ks.AddAesKey(rrNsName, keyName, keyValue)
}

func (ks *AESKeyStore) removeKeyFile(path string) {
	rrNsName, keyName, err := parseFileName(filepath.Base(path))
	if err != nil {
		return
	}

	ks.RemoveAesKey(rrNsName, keyName)
}

func parseFileName(fileName string) (types.NamespacedName, string, error) {
	parts := strings.Split(fileName, ".")
	// expected parts: [namespace, name, keyFile]
	if len(parts) != 3 {
		return types.NamespacedName{}, "", fmt.Errorf("path does not match expected format")
	}

	namespace := parts[0]
	name := parts[1]
	keyFileName := parts[2]

	return types.NamespacedName{Namespace: namespace, Name: name}, keyFileName, nil

}
