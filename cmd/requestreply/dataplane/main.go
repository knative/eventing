package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
	"crypto/aes"
    "crypto/cipher"
    "crypto/des"
    "crypto/rc4"
    "encoding/base64"
    "errors"
    "strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

type proxiedRequest struct {
	received       time.Time
	responseWriter http.ResponseWriter
	replyEvent     chan *cloudevents.Event
}

type proxiedRequestMap struct {
	lock    sync.RWMutex
	entries map[string]*proxiedRequest
}

func (m *proxiedRequestMap) addEvent(responseWriter http.ResponseWriter, event *cloudevents.Event) *proxiedRequest {
	m.lock.Lock()
	defer m.lock.Unlock()

	id := event.ID()
	pr := &proxiedRequest{
		received:       time.Now(),
		responseWriter: responseWriter,
		replyEvent:     make(chan *cloudevents.Event, 1),
	}
	m.entries[id] = pr

	return pr
}

func (m *proxiedRequestMap) deleteEvent(event *cloudevents.Event) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.entries, event.ID())
}

func determineEncryptionFunc(originalId string, key []byte, algorithm string) (string, error) { //Encrypt instead of Decrypt
	switch strings.ToUpper(string(algorithm)) {
	case "AES", "AES-ECB":
		return encryptWithAES(originalId, key)
	case "DES":
		return encryptWithDES(originalId, key)
	case "3DES", "TRIPLEDES":
		return encryptWithTripleDES(originalId, key)
	case "RC4":
		return encryptWithRC4(originalId, key)
	default:
		return "", errors.New("encryption failed")
	}
}

func encryptWithAES(originalId string, key []byte) (string, error) {
	block, err := aes.NewCipher(key) //initialize new cipher with the key (should be 16,24,32 bytes)
	if err != nil {
		return "", err
	}

	plaintext := []byte(originalId)
	blockSize := block.BlockSize() // 16
	padding := blockSize - (len(plaintext) % blockSize) // calculate padding bytes
	paddedText := make([]byte, len(plaintext)+padding) // new array combining original data & padding
	copy(paddedText, plaintext)

	for i := len(plaintext); i < len(paddedText); i++ {
		paddedText[i] = byte(padding)
	}
		
	// Encrypt
	ciphertext := make([]byte, len(paddedText))
	for i := 0; i < len(paddedText); i += blockSize {
		block.Encrypt(ciphertext[i:i+blockSize], paddedText[i:i+blockSize]) //ECB mode
	}
		
	// Return base64 encoded string
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func encryptWithDES(originalId string, key []byte) (string, error) {
	block, err := des.NewCipher(key)
    if err != nil {
        return "", err
    }
	
	plaintext := []byte(originalId)
    blockSize := block.BlockSize()
    padding := blockSize - (len(plaintext) % blockSize)
    paddedText := make([]byte, len(plaintext)+padding)
    copy(paddedText, plaintext)

	for i := len(plaintext); i < len(paddedText); i++ {
        paddedText[i] = byte(padding)
    }

	ciphertext := make([]byte, len(paddedText))
    for i := 0; i < len(paddedText); i += blockSize {
        block.Encrypt(ciphertext[i:i+blockSize], paddedText[i:i+blockSize])
    }

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func encryptWithTripleDES(originalId string, key []byte) (string, error) {
	block, err := des.NewTripleDESCipher(key)
    if err != nil {
        return "", err
    }

	plaintext := []byte(originalId)
    blockSize := block.BlockSize()
    padding := blockSize - (len(plaintext) % blockSize)
    paddedText := make([]byte, len(plaintext)+padding)
    copy(paddedText, plaintext)

	for i := len(plaintext); i < len(paddedText); i++ {
        paddedText[i] = byte(padding)
    }

	ciphertext := make([]byte, len(paddedText))
    for i := 0; i < len(paddedText); i += blockSize {
        block.Encrypt(ciphertext[i:i+blockSize], paddedText[i:i+blockSize])
    }

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func encryptWithRC4(originalId string, key []byte) (string, error) {
	cipher, err := rc4.NewCipher(key)
    if err != nil {
        return "", err
    }

	plaintext := []byte(originalId)
    ciphertext := make([]byte, len(plaintext))
    cipher.XORKeyStream(ciphertext, plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (m *proxiedRequestMap) HandleNewEvent(ctx context.Context, responseWriter http.ResponseWriter, event *cloudevents.Event) {
	fmt.Printf("handling new event: %s\n", event.String())

	key := []byte("placeholderkey")
	algorithm := "AES"

	originalID := event.ID() //generate signed id using event id
	signedID, err := encryptId(originalID, key, algorithm)
	if err != nil {
        fmt.Printf("failed to encrypt ID: %s\n", err)
        responseWriter.WriteHeader(http.StatusInternalServerError)
        return
    }
    correlationID := fmt.Sprintf("%s:%s", originalID, signedID)
    event.SetExtension("correlationId", correlationID)

	pr := m.addEvent(responseWriter, event)

	c, err := cloudevents.NewClientHTTP(cehttp.WithTarget(os.Getenv("K_SINK")))
	if err != nil {
		fmt.Printf("failed to start a client: %s\n", err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	res := c.Send(ctx, *event)
	if protocol.IsNACK(res) || protocol.IsUndelivered(res) {
		fmt.Printf("failed to send event %s: %s\n", event.ID(), res.Error())
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	for {
		select {
		case resp := <-pr.replyEvent:
			fmt.Printf("received a response event to event %s, sending back as response\n", event.ID())
			msg := binding.ToMessage(resp)
			err := cehttp.WriteResponseWriter(ctx, msg, http.StatusOK, pr.responseWriter)
			if err != nil {
				fmt.Printf("failed to send event back: %s\n", err.Error())
			}
			msg.Finish(err)
			m.deleteEvent(event)
			return
		case <-ctx.Done():
			fmt.Printf("context timeout reached before encountering a response to the event, discarding event %s\n", event.ID())
			responseWriter.WriteHeader(http.StatusRequestTimeout) // handle timeout to send http408
			m.deleteEvent(event)
			return
		}
	}
}
func (m *proxiedRequestMap) HandleResponseEvent(ctx context.Context, responseWriter http.ResponseWriter, event *cloudevents.Event) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	fmt.Printf("handling a response event!\n")

	responseWriter.WriteHeader(http.StatusAccepted)

	id := event.Extensions()["replyAttribute"] //check if event has replyAttribute
	pr, ok := m.entries[id.(string)]
	if !ok {
		fmt.Printf("no event found corresponding to the response id %s, discarding\n", id)
		return
	}

	// send the reply event back to the original response writer
	fmt.Printf("found an event corresponding to the response id %s, replying\n", id)
	pr.replyEvent <- event
}

func isResponseEvent(event *cloudevents.Event) bool {
	
	_, ok := event.Extensions()["replyAttribute"] //checks if event has replyAttribute
	return ok
}

func (m *proxiedRequestMap) HandleRequest(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("received a new request, handling it!")
	event, err := cloudevents.NewEventFromHTTPRequest(req)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if isResponseEvent(event) {
		fmt.Printf("received a response event, handling it!\n")
		m.HandleResponseEvent(ctx, w, event)
		fmt.Printf("finished handling response event\n")
	} else {
		fmt.Printf("received a new event, handling it!\n")
		m.HandleNewEvent(ctx, w, event)
		fmt.Printf("finished handling new event\n")
	}
}

func main() {
	proxyMap := &proxiedRequestMap{entries: make(map[string]*proxiedRequest)}
	http.HandleFunc("/", proxyMap.HandleRequest)

	http.ListenAndServe(":8080", nil)
}