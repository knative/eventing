package codec

import (
	"encoding/json"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec"
	"log"
	"strconv"
)

func JsonEncodeV01(e cloudevents.Event) ([]byte, error) {
	ctx := e.Context.AsV01()
	if ctx.ContentType == nil {
		ctx.ContentType = cloudevents.StringOfApplicationJSON()
	}
	return jsonEncode(ctx, e.Data)
}

func JsonEncodeV02(e cloudevents.Event) ([]byte, error) {
	ctx := e.Context.AsV02()
	if ctx.ContentType == nil {
		ctx.ContentType = cloudevents.StringOfApplicationJSON()
	}
	return jsonEncode(ctx, e.Data)
}

func JsonEncodeV03(e cloudevents.Event) ([]byte, error) {
	ctx := e.Context.AsV03()
	if ctx.DataContentType == nil {
		ctx.DataContentType = cloudevents.StringOfApplicationJSON()
	}
	return jsonEncode(ctx, e.Data)
}

func jsonEncode(ctx cloudevents.EventContext, data interface{}) ([]byte, error) {
	ctxb, err := marshalEvent(ctx)
	if err != nil {
		return nil, err
	}

	var body []byte

	b := map[string]json.RawMessage{}
	if err := json.Unmarshal(ctxb, &b); err != nil {
		return nil, err
	}

	mediaType := ctx.GetDataMediaType()
	datab, err := marshalEventData(mediaType, data)
	if err != nil {
		return nil, err
	}
	if data != nil {
		if mediaType == "" || mediaType == cloudevents.ApplicationJSON {
			b["data"] = datab
		} else if datab[0] != byte('"') {
			b["data"] = []byte(strconv.QuoteToASCII(string(datab)))
		} else {
			// already quoted
			b["data"] = datab
		}
	}

	body, err = json.Marshal(b)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func JsonDecodeV01(body []byte) (*cloudevents.Event, error) {
	ec := cloudevents.EventContextV01{}
	if err := json.Unmarshal(body, &ec); err != nil {
		return nil, err
	}

	raw := make(map[string]json.RawMessage)

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	var data interface{}
	if d, ok := raw["data"]; ok {
		data = []byte(d)
	}

	return &cloudevents.Event{
		Context: ec,
		Data:    data,
	}, nil
}

func JsonDecodeV02(body []byte) (*cloudevents.Event, error) {
	ec := cloudevents.EventContextV02{}
	if err := json.Unmarshal(body, &ec); err != nil {
		return nil, err
	}

	raw := make(map[string]json.RawMessage)

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	var data interface{}
	if d, ok := raw["data"]; ok {
		data = []byte(d)
	}

	return &cloudevents.Event{
		Context: ec,
		Data:    data,
	}, nil
}

func JsonDecodeV03(body []byte) (*cloudevents.Event, error) {
	ec := cloudevents.EventContextV03{}
	if err := json.Unmarshal(body, &ec); err != nil {
		return nil, err
	}

	raw := make(map[string]json.RawMessage)

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	var data interface{}
	if d, ok := raw["data"]; ok {
		data = []byte(d)
	}

	return &cloudevents.Event{
		Context: ec,
		Data:    data,
	}, nil
}

func marshalEvent(event interface{}) ([]byte, error) {
	if b, ok := event.([]byte); ok {
		log.Printf("json.marshalEvent asked to encode bytes... wrong? %s", string(b))
	}

	b, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// TODO: not sure about this location for eventdata.
func marshalEventData(encoding string, data interface{}) ([]byte, error) {
	if data == nil {
		return []byte(nil), nil
	}
	// already encoded?
	if b, ok := data.([]byte); ok {
		return b, nil
	}
	return datacodec.Encode(encoding, data)
}
