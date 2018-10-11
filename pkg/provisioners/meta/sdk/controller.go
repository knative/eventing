package sdk

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"log"
	"net/http"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

var (
	port    int
	ctx     context.Context
	handler Handler
)

type Handler interface {
	Convert(context.Context, string, []byte) (runtime.Object, error)
	Handle(context.Context, eventingv1alpha1.Source, []runtime.Object) (*eventingv1alpha1.SourceStatus, []runtime.Object, error)
}

type SyncRequest struct {
	Controller *runtime.RawExtension    `json:"controller"`
	Parent     eventingv1alpha1.Source  `json:"parent"`
	Children   map[string]ChildrenGroup `json:"children"` // TODO not sure how to deal with children
	Finalizing bool                     `json:"finalizing"`
}

type ChildrenGroup map[string]unstructured.Unstructured

type SyncResponse struct {
	Status   eventingv1alpha1.SourceStatus `json:"status"`
	Children []runtime.Object              `json:"children"`
}

func Handle(h Handler) {
	handler = h
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)

	log.Printf("Sync: %s", string(body))

	if err != nil {
		log.Printf("Error: failed to read all body, %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	request := &SyncRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		log.Printf("Error: failed to unmarshal  request, %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	children := make([]runtime.Object, 0, len(request.Children)) // this is wrong, .children is the size of the map, not the objects. But it will scale up

	for k, cg := range request.Children {
		for _, child := range cg {
			cjson, err := child.MarshalJSON()
			if err != nil {
				log.Printf("Error: failed to marshal json from child %v, %v", child, err)
				continue
			}
			newChild, err := handler.Convert(ctx, k, cjson)
			if err != nil {
				log.Printf("Error: failed to convert child json %q to runtime object, %v", string(cjson), err)
				continue
			}
			log.Printf("converted a child: %v", newChild)
			children = append(children, newChild)
		}
	}

	status, newChildren, err := handler.Handle(ctx, request.Parent, children) // TODO convert the children to a flat list.
	if err != nil {
		log.Printf("Error: failed to handle request, %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, err = json.Marshal(&SyncResponse{
		Status:   *status,
		Children: newChildren,
	})
	if err != nil {
		log.Printf("Error: failed to marshel response, %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func Run(c context.Context) {
	flag.Parse()

	ctx = c
	http.HandleFunc("/", httpHandler)

	server := &http.Server{Addr: fmt.Sprintf(":%d", port)}
	go func() {
		log.Fatal(server.ListenAndServe())
	}()

	// Shutdown on SIGTERM.
	sig := <-ctx.Done()
	log.Printf("Received %v signal. Shutting down...", sig)
	server.Shutdown(ctx)
}

func init() {
	flag.IntVar(&port, "port", 80, "The port to listen on")
}
