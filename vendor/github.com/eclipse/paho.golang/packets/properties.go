package packets

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// PropPayloadFormat, etc are the list of property codes for the
// MQTT packet properties
const (
	PropPayloadFormat          byte = 1
	PropMessageExpiry          byte = 2
	PropContentType            byte = 3
	PropResponseTopic          byte = 8
	PropCorrelationData        byte = 9
	PropSubscriptionIdentifier byte = 11
	PropSessionExpiryInterval  byte = 17
	PropAssignedClientID       byte = 18
	PropServerKeepAlive        byte = 19
	PropAuthMethod             byte = 21
	PropAuthData               byte = 22
	PropRequestProblemInfo     byte = 23
	PropWillDelayInterval      byte = 24
	PropRequestResponseInfo    byte = 25
	PropResponseInfo           byte = 26
	PropServerReference        byte = 28
	PropReasonString           byte = 31
	PropReceiveMaximum         byte = 33
	PropTopicAliasMaximum      byte = 34
	PropTopicAlias             byte = 35
	PropMaximumQOS             byte = 36
	PropRetainAvailable        byte = 37
	PropUser                   byte = 38
	PropMaximumPacketSize      byte = 39
	PropWildcardSubAvailable   byte = 40
	PropSubIDAvailable         byte = 41
	PropSharedSubAvailable     byte = 42
)

// User is a struct for the User properties, originally it was a map
// then it was pointed out that user properties are allowed to appear
// more than once
type User struct {
	Key, Value string
}

// Properties is a struct representing the all the described properties
// allowed by the MQTT protocol, determining the validity of a property
// relvative to the packettype it was received in is provided by the
// ValidateID function
type Properties struct {
	// PayloadFormat indicates the format of the payload of the message
	// 0 is unspecified bytes
	// 1 is UTF8 encoded character data
	PayloadFormat *byte
	// MessageExpiry is the lifetime of the message in seconds
	MessageExpiry *uint32
	// ContentType is a UTF8 string describing the content of the message
	// for example it could be a MIME type
	ContentType string
	// ResponseTopic is a UTF8 string indicating the topic name to which any
	// response to this message should be sent
	ResponseTopic string
	// CorrelationData is binary data used to associate future response
	// messages with the original request message
	CorrelationData []byte
	// SubscriptionIdentifier is an identifier of the subscription to which
	// the Publish matched
	SubscriptionIdentifier *int
	// SessionExpiryInterval is the time in seconds after a client disconnects
	// that the server should retain the session information (subscriptions etc)
	SessionExpiryInterval *uint32
	// AssignedClientID is the server assigned client identifier in the case
	// that a client connected without specifying a clientID the server
	// generates one and returns it in the Connack
	AssignedClientID string
	// ServerKeepAlive allows the server to specify in the Connack packet
	// the time in seconds to be used as the keep alive value
	ServerKeepAlive *uint16
	// AuthMethod is a UTF8 string containing the name of the authentication
	// method to be used for extended authentication
	AuthMethod string
	// AuthData is binary data containing authentication data
	AuthData []byte
	// RequestProblemInfo is used by the Client to indicate to the server to
	// include the Reason String and/or User Properties in case of failures
	RequestProblemInfo *byte
	// WillDelayInterval is the number of seconds the server waits after the
	// point at which it would otherwise send the will message before sending
	// it. The client reconnecting before that time expires causes the server
	// to cancel sending the will
	WillDelayInterval *uint32
	// RequestResponseInfo is used by the Client to request the Server provide
	// Response Information in the Connack
	RequestResponseInfo *byte
	// ResponseInfo is a UTF8 encoded string that can be used as the basis for
	// createing a Response Topic. The way in which the Client creates a
	// Response Topic from the Response Information is not defined. A common
	// use of this is to pass a globally unique portion of the topic tree which
	// is reserved for this Client for at least the lifetime of its Session. This
	// often cannot just be a random name as both the requesting Client and the
	// responding Client need to be authorized to use it. It is normal to use this
	// as the root of a topic tree for a particular Client. For the Server to
	// return this information, it normally needs to be correctly configured.
	// Using this mechanism allows this configuration to be done once in the
	// Server rather than in each Client
	ResponseInfo string
	// ServerReference is a UTF8 string indicating another server the client
	// can use
	ServerReference string
	// ReasonString is a UTF8 string representing the reason associated with
	// this response, intended to be human readable for diagnostic purposes
	ReasonString string
	// ReceiveMaximum is the maximum number of QOS1 & 2 messages allowed to be
	// 'inflight' (not having received a PUBACK/PUBCOMP response for)
	ReceiveMaximum *uint16
	// TopicAliasMaximum is the highest value permitted as a Topic Alias
	TopicAliasMaximum *uint16
	// TopicAlias is used in place of the topic string to reduce the size of
	// packets for repeated messages on a topic
	TopicAlias *uint16
	// MaximumQOS is the highest QOS level permitted for a Publish
	MaximumQOS *byte
	// RetainAvailable indicates whether the server supports messages with the
	// retain flag set
	RetainAvailable *byte
	// User is a slice of user provided properties (key and value)
	User []User
	// MaximumPacketSize allows the client or server to specify the maximum packet
	// size in bytes that they support
	MaximumPacketSize *uint32
	// WildcardSubAvailable indicates whether wildcard subscriptions are permitted
	WildcardSubAvailable *byte
	// SubIDAvailable indicates whether subscription identifiers are supported
	SubIDAvailable *byte
	// SharedSubAvailable indicates whether shared subscriptions are supported
	SharedSubAvailable *byte
}

func (p *Properties) String() string {
	var b strings.Builder
	if p.PayloadFormat != nil {
		fmt.Fprintf(&b, "\tPayloadFormat:%d\n", *p.PayloadFormat)
	}
	if p.MessageExpiry != nil {
		fmt.Fprintf(&b, "\tMessageExpiry:%d\n", *p.MessageExpiry)
	}
	if p.ContentType != "" {
		fmt.Fprintf(&b, "\tContentType:%s\n", p.ContentType)
	}
	if p.ResponseTopic != "" {
		fmt.Fprintf(&b, "\tResponseTopic:%s\n", p.ResponseTopic)
	}
	if len(p.CorrelationData) > 0 {
		fmt.Fprintf(&b, "\tCorrelationData:%X\n", p.CorrelationData)
	}
	if p.SubscriptionIdentifier != nil {
		fmt.Fprintf(&b, "\tSubscriptionIdentifier:%d\n", *p.SubscriptionIdentifier)
	}
	if p.SessionExpiryInterval != nil {
		fmt.Fprintf(&b, "\tSessionExpiryInterval:%d\n", *p.SessionExpiryInterval)
	}
	if p.AssignedClientID != "" {
		fmt.Fprintf(&b, "\tAssignedClientID:%s\n", p.AssignedClientID)
	}
	if p.ServerKeepAlive != nil {
		fmt.Fprintf(&b, "\tServerKeepAlive:%d\n", *p.ServerKeepAlive)
	}
	if p.AuthMethod != "" {
		fmt.Fprintf(&b, "\tAuthMethod:%s\n", p.AuthMethod)
	}
	if len(p.AuthData) > 0 {
		fmt.Fprintf(&b, "\tAuthData:%X\n", p.AuthData)
	}
	if p.RequestProblemInfo != nil {
		fmt.Fprintf(&b, "\tRequestProblemInfo:%d\n", *p.RequestProblemInfo)
	}
	if p.WillDelayInterval != nil {
		fmt.Fprintf(&b, "\tWillDelayInterval:%d\n", *p.WillDelayInterval)
	}
	if p.RequestResponseInfo != nil {
		fmt.Fprintf(&b, "\tRequestResponseInfo:%d\n", *p.RequestResponseInfo)
	}
	if p.ServerReference != "" {
		fmt.Fprintf(&b, "\tServerReference:%s\n", p.ServerReference)
	}
	if p.ReasonString != "" {
		fmt.Fprintf(&b, "\tReasonString:%s\n", p.ReasonString)
	}
	if p.ReceiveMaximum != nil {
		fmt.Fprintf(&b, "\tReceiveMaximum:%d\n", *p.ReceiveMaximum)
	}
	if p.TopicAliasMaximum != nil {
		fmt.Fprintf(&b, "\tTopicAliasMaximum:%d\n", *p.TopicAliasMaximum)
	}
	if p.TopicAlias != nil {
		fmt.Fprintf(&b, "\tTopicAlias:%d\n", *p.TopicAlias)
	}
	if p.MaximumQOS != nil {
		fmt.Fprintf(&b, "\tMaximumQOS:%d\n", *p.MaximumQOS)
	}
	if p.RetainAvailable != nil {
		fmt.Fprintf(&b, "\tRetainAvailable:%d\n", *p.RetainAvailable)
	}
	if p.MaximumPacketSize != nil {
		fmt.Fprintf(&b, "\tMaximumPacketSize:%d\n", *p.MaximumPacketSize)
	}
	if p.WildcardSubAvailable != nil {
		fmt.Fprintf(&b, "\tWildcardSubAvailable:%d\n", *p.WildcardSubAvailable)
	}
	if p.SubIDAvailable != nil {
		fmt.Fprintf(&b, "\tSubIDAvailable:%d\n", *p.SubIDAvailable)
	}
	if p.SharedSubAvailable != nil {
		fmt.Fprintf(&b, "\tSharedSubAvailable:%d\n", *p.SharedSubAvailable)
	}
	if len(p.User) > 0 {
		fmt.Fprint(&b, "\tUser Properties:\n")
		for _, v := range p.User {
			fmt.Fprintf(&b, "\t\t%s:%s\n", v.Key, v.Value)
		}
	}

	return b.String()
}

// Pack takes all the defined properties for an Properties and produces
// a slice of bytes representing the wire format for the information
func (i *Properties) Pack(p byte) []byte {
	var b bytes.Buffer

	if i == nil {
		return nil
	}

	if p == PUBLISH {
		if i.PayloadFormat != nil {
			b.WriteByte(PropPayloadFormat)
			b.WriteByte(*i.PayloadFormat)
		}

		if i.MessageExpiry != nil {
			b.WriteByte(PropMessageExpiry)
			writeUint32(*i.MessageExpiry, &b)
		}

		if i.ContentType != "" {
			b.WriteByte(PropContentType)
			writeString(i.ContentType, &b)
		}

		if i.ResponseTopic != "" {
			b.WriteByte(PropResponseTopic)
			writeString(i.ResponseTopic, &b)
		}

		if len(i.CorrelationData) > 0 {
			b.WriteByte(PropCorrelationData)
			writeBinary(i.CorrelationData, &b)
		}

		if i.TopicAlias != nil {
			b.WriteByte(PropTopicAlias)
			writeUint16(*i.TopicAlias, &b)
		}
	}

	if p == PUBLISH || p == SUBSCRIBE {
		if i.SubscriptionIdentifier != nil {
			b.WriteByte(PropSubscriptionIdentifier)
			encodeVBIdirect(*i.SubscriptionIdentifier, &b)
		}
	}

	if p == CONNECT || p == CONNACK {
		if i.ReceiveMaximum != nil {
			b.WriteByte(PropReceiveMaximum)
			writeUint16(*i.ReceiveMaximum, &b)
		}

		if i.TopicAliasMaximum != nil {
			b.WriteByte(PropTopicAliasMaximum)
			writeUint16(*i.TopicAliasMaximum, &b)
		}

		if i.MaximumQOS != nil {
			b.WriteByte(PropMaximumQOS)
			b.WriteByte(*i.MaximumQOS)
		}

		if i.MaximumPacketSize != nil {
			b.WriteByte(PropMaximumPacketSize)
			writeUint32(*i.MaximumPacketSize, &b)
		}
	}

	if p == CONNACK {
		if i.AssignedClientID != "" {
			b.WriteByte(PropAssignedClientID)
			writeString(i.AssignedClientID, &b)
		}

		if i.ServerKeepAlive != nil {
			b.WriteByte(PropServerKeepAlive)
			writeUint16(*i.ServerKeepAlive, &b)
		}

		if i.WildcardSubAvailable != nil {
			b.WriteByte(PropWildcardSubAvailable)
			b.WriteByte(*i.WildcardSubAvailable)
		}

		if i.SubIDAvailable != nil {
			b.WriteByte(PropSubIDAvailable)
			b.WriteByte(*i.SubIDAvailable)
		}

		if i.SharedSubAvailable != nil {
			b.WriteByte(PropSharedSubAvailable)
			b.WriteByte(*i.SharedSubAvailable)
		}

		if i.RetainAvailable != nil {
			b.WriteByte(PropRetainAvailable)
			b.WriteByte(*i.RetainAvailable)
		}

		if i.ResponseInfo != "" {
			b.WriteByte(PropResponseInfo)
			writeString(i.ResponseInfo, &b)
		}
	}

	if p == CONNECT {
		if i.RequestProblemInfo != nil {
			b.WriteByte(PropRequestProblemInfo)
			b.WriteByte(*i.RequestProblemInfo)
		}

		if i.WillDelayInterval != nil {
			b.WriteByte(PropWillDelayInterval)
			writeUint32(*i.WillDelayInterval, &b)
		}

		if i.RequestResponseInfo != nil {
			b.WriteByte(PropRequestResponseInfo)
			b.WriteByte(*i.RequestResponseInfo)
		}
	}

	if p == CONNECT || p == CONNACK || p == DISCONNECT {
		if i.SessionExpiryInterval != nil {
			b.WriteByte(PropSessionExpiryInterval)
			writeUint32(*i.SessionExpiryInterval, &b)
		}
	}

	if p == CONNECT || p == CONNACK || p == AUTH {
		if i.AuthMethod != "" {
			b.WriteByte(PropAuthMethod)
			writeString(i.AuthMethod, &b)
		}

		if i.AuthData != nil && len(i.AuthData) > 0 {
			b.WriteByte(PropAuthData)
			writeBinary(i.AuthData, &b)
		}
	}

	if p == CONNACK || p == DISCONNECT {
		if i.ServerReference != "" {
			b.WriteByte(PropServerReference)
			writeString(i.ServerReference, &b)
		}
	}

	if p != CONNECT {
		if i.ReasonString != "" {
			b.WriteByte(PropReasonString)
			writeString(i.ReasonString, &b)
		}
	}

	for _, v := range i.User {
		b.WriteByte(PropUser)
		writeString(v.Key, &b)
		writeString(v.Value, &b)
	}

	return b.Bytes()
}

// PackBuf will create a bytes.Buffer of the packed properties, it
// will only pack the properties appropriate to the packet type p
// even though other properties may exist, it will silently ignore
// them
func (i *Properties) PackBuf(p byte) *bytes.Buffer {
	var b bytes.Buffer

	if i == nil {
		return nil
	}

	if p == PUBLISH {
		if i.PayloadFormat != nil {
			b.WriteByte(PropPayloadFormat)
			b.WriteByte(*i.PayloadFormat)
		}

		if i.MessageExpiry != nil {
			b.WriteByte(PropMessageExpiry)
			writeUint32(*i.MessageExpiry, &b)
		}

		if i.ContentType != "" {
			b.WriteByte(PropContentType)
			writeString(i.ContentType, &b)
		}

		if i.ResponseTopic != "" {
			b.WriteByte(PropResponseTopic)
			writeString(i.ResponseTopic, &b)
		}

		if i.CorrelationData != nil && len(i.CorrelationData) > 0 {
			b.WriteByte(PropCorrelationData)
			writeBinary(i.CorrelationData, &b)
		}

		if i.TopicAlias != nil {
			b.WriteByte(PropTopicAlias)
			writeUint16(*i.TopicAlias, &b)
		}
	}

	if p == PUBLISH || p == SUBSCRIBE {
		if i.SubscriptionIdentifier != nil {
			b.WriteByte(PropSubscriptionIdentifier)
			encodeVBIdirect(*i.SubscriptionIdentifier, &b)
		}
	}

	if p == CONNECT || p == CONNACK {
		if i.ReceiveMaximum != nil {
			b.WriteByte(PropReceiveMaximum)
			writeUint16(*i.ReceiveMaximum, &b)
		}

		if i.TopicAliasMaximum != nil {
			b.WriteByte(PropTopicAliasMaximum)
			writeUint16(*i.TopicAliasMaximum, &b)
		}

		if i.MaximumQOS != nil {
			b.WriteByte(PropMaximumQOS)
			b.WriteByte(*i.MaximumQOS)
		}

		if i.MaximumPacketSize != nil {
			b.WriteByte(PropMaximumPacketSize)
			writeUint32(*i.MaximumPacketSize, &b)
		}
	}

	if p == CONNACK {
		if i.AssignedClientID != "" {
			b.WriteByte(PropAssignedClientID)
			writeString(i.AssignedClientID, &b)
		}

		if i.ServerKeepAlive != nil {
			b.WriteByte(PropServerKeepAlive)
			writeUint16(*i.ServerKeepAlive, &b)
		}

		if i.WildcardSubAvailable != nil {
			b.WriteByte(PropWildcardSubAvailable)
			b.WriteByte(*i.WildcardSubAvailable)
		}

		if i.SubIDAvailable != nil {
			b.WriteByte(PropSubIDAvailable)
			b.WriteByte(*i.SubIDAvailable)
		}

		if i.SharedSubAvailable != nil {
			b.WriteByte(PropSharedSubAvailable)
			b.WriteByte(*i.SharedSubAvailable)
		}

		if i.RetainAvailable != nil {
			b.WriteByte(PropRetainAvailable)
			b.WriteByte(*i.RetainAvailable)
		}

		if i.ResponseInfo != "" {
			b.WriteByte(PropResponseInfo)
			writeString(i.ResponseInfo, &b)
		}
	}

	if p == CONNECT {
		if i.RequestProblemInfo != nil {
			b.WriteByte(PropRequestProblemInfo)
			b.WriteByte(*i.RequestProblemInfo)
		}

		if i.WillDelayInterval != nil {
			b.WriteByte(PropWillDelayInterval)
			writeUint32(*i.WillDelayInterval, &b)
		}

		if i.RequestResponseInfo != nil {
			b.WriteByte(PropRequestResponseInfo)
			b.WriteByte(*i.RequestResponseInfo)
		}
	}

	if p == CONNECT || p == CONNACK || p == DISCONNECT {
		if i.SessionExpiryInterval != nil {
			b.WriteByte(PropSessionExpiryInterval)
			writeUint32(*i.SessionExpiryInterval, &b)
		}
	}

	if p == CONNECT || p == CONNACK || p == AUTH {
		if i.AuthMethod != "" {
			b.WriteByte(PropAuthMethod)
			writeString(i.AuthMethod, &b)
		}

		if i.AuthData != nil && len(i.AuthData) > 0 {
			b.WriteByte(PropAuthData)
			writeBinary(i.AuthData, &b)
		}
	}

	if p == CONNACK || p == DISCONNECT {
		if i.ServerReference != "" {
			b.WriteByte(PropServerReference)
			writeString(i.ServerReference, &b)
		}
	}

	if p != CONNECT {
		if i.ReasonString != "" {
			b.WriteByte(PropReasonString)
			writeString(i.ReasonString, &b)
		}
	}

	for _, v := range i.User {
		b.WriteByte(PropUser)
		writeString(v.Key, &b)
		writeString(v.Value, &b)
	}

	return &b
}

// Unpack takes a buffer of bytes and reads out the defined properties
// filling in the appropriate entries in the struct, it returns the number
// of bytes used to store the Prop data and any error in decoding them
func (i *Properties) Unpack(r *bytes.Buffer, p byte) error {
	vbi, err := getVBI(r)
	if err != nil {
		return err
	}
	size, err := decodeVBI(vbi)
	if err != nil {
		return err
	}
	if size == 0 {
		return nil
	}

	buf := bytes.NewBuffer(r.Next(size))
	for {
		PropType, err := buf.ReadByte()
		if err != nil && err != io.EOF {
			return err
		}
		if err == io.EOF {
			break
		}
		if !ValidateID(p, PropType) {
			return fmt.Errorf("invalid Prop type %d for packet %d", PropType, p)
		}
		switch PropType {
		case PropPayloadFormat:
			pf, err := buf.ReadByte()
			if err != nil {
				return err
			}
			i.PayloadFormat = &pf
		case PropMessageExpiry:
			pe, err := readUint32(buf)
			if err != nil {
				return err
			}
			i.MessageExpiry = &pe
		case PropContentType:
			ct, err := readString(buf)
			if err != nil {
				return err
			}
			i.ContentType = ct
		case PropResponseTopic:
			tr, err := readString(buf)
			if err != nil {
				return err
			}
			i.ResponseTopic = tr
		case PropCorrelationData:
			cd, err := readBinary(buf)
			if err != nil {
				return err
			}
			i.CorrelationData = cd
		case PropSubscriptionIdentifier:
			si, err := decodeVBI(buf)
			if err != nil {
				return err
			}
			i.SubscriptionIdentifier = &si
		case PropSessionExpiryInterval:
			se, err := readUint32(buf)
			if err != nil {
				return err
			}
			i.SessionExpiryInterval = &se
		case PropAssignedClientID:
			ac, err := readString(buf)
			if err != nil {
				return err
			}
			i.AssignedClientID = ac
		case PropServerKeepAlive:
			sk, err := readUint16(buf)
			if err != nil {
				return err
			}
			i.ServerKeepAlive = &sk
		case PropAuthMethod:
			am, err := readString(buf)
			if err != nil {
				return err
			}
			i.AuthMethod = am
		case PropAuthData:
			ad, err := readBinary(buf)
			if err != nil {
				return err
			}
			i.AuthData = ad
		case PropRequestProblemInfo:
			rp, err := buf.ReadByte()
			if err != nil {
				return err
			}
			i.RequestProblemInfo = &rp
		case PropWillDelayInterval:
			wd, err := readUint32(buf)
			if err != nil {
				return err
			}
			i.WillDelayInterval = &wd
		case PropRequestResponseInfo:
			rp, err := buf.ReadByte()
			if err != nil {
				return err
			}
			i.RequestResponseInfo = &rp
		case PropResponseInfo:
			ri, err := readString(buf)
			if err != nil {
				return err
			}
			i.ResponseInfo = ri
		case PropServerReference:
			sr, err := readString(buf)
			if err != nil {
				return err
			}
			i.ServerReference = sr
		case PropReasonString:
			rs, err := readString(buf)
			if err != nil {
				return err
			}
			i.ReasonString = rs
		case PropReceiveMaximum:
			rm, err := readUint16(buf)
			if err != nil {
				return err
			}
			i.ReceiveMaximum = &rm
		case PropTopicAliasMaximum:
			ta, err := readUint16(buf)
			if err != nil {
				return err
			}
			i.TopicAliasMaximum = &ta
		case PropTopicAlias:
			ta, err := readUint16(buf)
			if err != nil {
				return err
			}
			i.TopicAlias = &ta
		case PropMaximumQOS:
			mq, err := buf.ReadByte()
			if err != nil {
				return err
			}
			i.MaximumQOS = &mq
		case PropRetainAvailable:
			ra, err := buf.ReadByte()
			if err != nil {
				return err
			}
			i.RetainAvailable = &ra
		case PropUser:
			k, err := readString(buf)
			if err != nil {
				return err
			}
			v, err := readString(buf)
			if err != nil {
				return err
			}
			i.User = append(i.User, User{k, v})
		case PropMaximumPacketSize:
			mp, err := readUint32(buf)
			if err != nil {
				return err
			}
			i.MaximumPacketSize = &mp
		case PropWildcardSubAvailable:
			ws, err := buf.ReadByte()
			if err != nil {
				return err
			}
			i.WildcardSubAvailable = &ws
		case PropSubIDAvailable:
			si, err := buf.ReadByte()
			if err != nil {
				return err
			}
			i.SubIDAvailable = &si
		case PropSharedSubAvailable:
			ss, err := buf.ReadByte()
			if err != nil {
				return err
			}
			i.SharedSubAvailable = &ss
		default:
			return fmt.Errorf("unknown Prop type %d", PropType)
		}
	}

	return nil
}

// ValidProperties is a map of the various properties and the
// PacketTypes that property is valid for.
// A CONNECT packet has own properties, but may also include a separate set of Will Properties.
// Currently, `CONNECT` covers both sets, this may lead to some invalid properties being accepted (this may be fixed in the future).
var ValidProperties = map[byte]map[byte]struct{}{
	PropPayloadFormat:          {CONNECT: {}, PUBLISH: {}},
	PropMessageExpiry:          {CONNECT: {}, PUBLISH: {}},
	PropContentType:            {CONNECT: {}, PUBLISH: {}},
	PropResponseTopic:          {CONNECT: {}, PUBLISH: {}},
	PropCorrelationData:        {CONNECT: {}, PUBLISH: {}},
	PropTopicAlias:             {PUBLISH: {}},
	PropSubscriptionIdentifier: {PUBLISH: {}, SUBSCRIBE: {}},
	PropSessionExpiryInterval:  {CONNECT: {}, CONNACK: {}, DISCONNECT: {}},
	PropAssignedClientID:       {CONNACK: {}},
	PropServerKeepAlive:        {CONNACK: {}},
	PropWildcardSubAvailable:   {CONNACK: {}},
	PropSubIDAvailable:         {CONNACK: {}},
	PropSharedSubAvailable:     {CONNACK: {}},
	PropRetainAvailable:        {CONNACK: {}},
	PropResponseInfo:           {CONNACK: {}},
	PropAuthMethod:             {CONNECT: {}, CONNACK: {}, AUTH: {}},
	PropAuthData:               {CONNECT: {}, CONNACK: {}, AUTH: {}},
	PropRequestProblemInfo:     {CONNECT: {}},
	PropWillDelayInterval:      {CONNECT: {}},
	PropRequestResponseInfo:    {CONNECT: {}},
	PropServerReference:        {CONNACK: {}, DISCONNECT: {}},
	PropReasonString:           {CONNACK: {}, PUBACK: {}, PUBREC: {}, PUBREL: {}, PUBCOMP: {}, SUBACK: {}, UNSUBACK: {}, DISCONNECT: {}, AUTH: {}},
	PropReceiveMaximum:         {CONNECT: {}, CONNACK: {}},
	PropTopicAliasMaximum:      {CONNECT: {}, CONNACK: {}},
	PropMaximumQOS:             {CONNECT: {}, CONNACK: {}},
	PropMaximumPacketSize:      {CONNECT: {}, CONNACK: {}},
	PropUser:                   {CONNECT: {}, CONNACK: {}, PUBLISH: {}, PUBACK: {}, PUBREC: {}, PUBREL: {}, PUBCOMP: {}, SUBSCRIBE: {}, UNSUBSCRIBE: {}, SUBACK: {}, UNSUBACK: {}, DISCONNECT: {}, AUTH: {}},
}

// ValidateID takes a PacketType and a property name and returns
// a boolean indicating if that property is valid for that
// PacketType
func ValidateID(p byte, i byte) bool {
	_, ok := ValidProperties[i][p]
	return ok
}
