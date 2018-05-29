package sources

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

func RunEventSource(es EventSource) {
	op := os.Getenv(BindOperationKey)
	bindContext := os.Getenv(BindContextKey)
	trigger := os.Getenv(BindTriggerKey)
	target := os.Getenv(BindTargetKey)

	decodedContext, _ := base64.StdEncoding.DecodeString(bindContext)
	decodedTrigger, _ := base64.StdEncoding.DecodeString(trigger)
	var c BindContext
	var t EventTrigger

	err := json.Unmarshal(decodedContext, &c)
	if err != nil {
		panic(fmt.Sprintf("can not unmarshal %q %q : %s", BindContextKey, decodedContext, err))
	}

	err = json.Unmarshal(decodedTrigger, &t)
	if err != nil {
		panic(fmt.Sprintf("can not unmarshal %q %q : %s", BindTriggerKey, decodedTrigger, err))
	}

	log.Printf("Doing %q target: %q Received bindContext as %+v trigger %+v", op, target, c, t)
	if op == string(Bind) {
		b, err := es.Bind(t, target)
		if err != nil {
			panic(fmt.Sprintf("Failed to bind: %s", err))
		}

		marshalledBindContext, err := json.Marshal(b)
		if err != nil {
			panic(fmt.Sprintf("Failed to marshal returned bind context %+v : %s", b, err))
		}
		encodedBindContext := base64.StdEncoding.EncodeToString(marshalledBindContext)

		err = ioutil.WriteFile("/dev/termination-log", []byte(encodedBindContext), os.ModeDevice)
		if err != nil {
			panic(fmt.Sprintf("Failed to write the termination message: %s", err))
		}
	} else {
		err = es.Unbind(t, c)
		if err != nil {
			panic(fmt.Sprintf("Failed to unbind: %s", err))
		}
	}
}
