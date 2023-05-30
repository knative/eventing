package eventingtlstesting

import "net/http"

// RequestsChannelHandler
func RequestsChannelHandler(requestsChan chan<- *http.Request) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if requestsChan != nil {
			requestsChan <- request
		}
		if request.TLS == nil {
			// It's not on TLS, fail request
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusOK)
	})
}
