package packets

import (
	"bytes"
	"fmt"
	"io"
	"net"
)

// Connack is the Variable Header definition for a connack control packet
type Connack struct {
	Properties     *Properties
	ReasonCode     byte
	SessionPresent bool
}

const (
	ConnackSuccess                     = 0x00
	ConnackUnspecifiedError            = 0x80
	ConnackMalformedPacket             = 0x81
	ConnackProtocolError               = 0x82
	ConnackImplementationSpecificError = 0x83
	ConnackUnsupportedProtocolVersion  = 0x84
	ConnackInvalidClientID             = 0x85
	ConnackBadUsernameOrPassword       = 0x86
	ConnackNotAuthorized               = 0x87
	ConnackServerUnavailable           = 0x88
	ConnackServerBusy                  = 0x89
	ConnackBanned                      = 0x8A
	ConnackBadAuthenticationMethod     = 0x8C
	ConnackTopicNameInvalid            = 0x90
	ConnackPacketTooLarge              = 0x95
	ConnackQuotaExceeded               = 0x97
	ConnackPayloadFormatInvalid        = 0x99
	ConnackRetainNotSupported          = 0x9A
	ConnackQoSNotSupported             = 0x9B
	ConnackUseAnotherServer            = 0x9C
	ConnackServerMoved                 = 0x9D
	ConnackConnectionRateExceeded      = 0x9F
)

func (c *Connack) String() string {
	return fmt.Sprintf("CONNACK: ReasonCode:%d SessionPresent:%t\nProperties:\n%s", c.ReasonCode, c.SessionPresent, c.Properties)
}

//Unpack is the implementation of the interface required function for a packet
func (c *Connack) Unpack(r *bytes.Buffer) error {
	connackFlags, err := r.ReadByte()
	if err != nil {
		return err
	}
	c.SessionPresent = connackFlags&0x01 > 0

	c.ReasonCode, err = r.ReadByte()
	if err != nil {
		return err
	}

	err = c.Properties.Unpack(r, CONNACK)
	if err != nil {
		return err
	}

	return nil
}

// Buffers is the implementation of the interface required function for a packet
func (c *Connack) Buffers() net.Buffers {
	var header bytes.Buffer

	if c.SessionPresent {
		header.WriteByte(1)
	} else {
		header.WriteByte(0)
	}
	header.WriteByte(c.ReasonCode)

	idvp := c.Properties.Pack(CONNACK)
	propLen := encodeVBI(len(idvp))

	n := net.Buffers{header.Bytes(), propLen}
	if len(idvp) > 0 {
		n = append(n, idvp)
	}

	return n
}

// WriteTo is the implementation of the interface required function for a packet
func (c *Connack) WriteTo(w io.Writer) (int64, error) {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: CONNACK}}
	cp.Content = c

	return cp.WriteTo(w)
}

// Reason returns a string representation of the meaning of the ReasonCode
func (c *Connack) Reason() string {
	switch c.ReasonCode {
	case 0:
		return "Success - The Connection is accepted."
	case 128:
		return "Unspecified error - The Server does not wish to reveal the reason for the failure, or none of the other Reason Codes apply."
	case 129:
		return "Malformed Packet - Data within the CONNECT packet could not be correctly parsed."
	case 130:
		return "Protocol Error - Data in the CONNECT packet does not conform to this specification."
	case 131:
		return "Implementation specific error - The CONNECT is valid but is not accepted by this Server."
	case 132:
		return "Unsupported Protocol Version - The Server does not support the version of the MQTT protocol requested by the Client."
	case 133:
		return "Client Identifier not valid - The Client Identifier is a valid string but is not allowed by the Server."
	case 134:
		return "Bad User Name or Password - The Server does not accept the User Name or Password specified by the Client"
	case 135:
		return "Not authorized - The Client is not authorized to connect."
	case 136:
		return "Server unavailable - The MQTT Server is not available."
	case 137:
		return "Server busy - The Server is busy. Try again later."
	case 138:
		return "Banned - This Client has been banned by administrative action. Contact the server administrator."
	case 140:
		return "Bad authentication method - The authentication method is not supported or does not match the authentication method currently in use."
	case 144:
		return "Topic Name invalid - The Will Topic Name is not malformed, but is not accepted by this Server."
	case 149:
		return "Packet too large - The CONNECT packet exceeded the maximum permissible size."
	case 151:
		return "Quota exceeded - An implementation or administrative imposed limit has been exceeded."
	case 154:
		return "Retain not supported - The Server does not support retained messages, and Will Retain was set to 1."
	case 155:
		return "QoS not supported - The Server does not support the QoS set in Will QoS."
	case 156:
		return "Use another server - The Client should temporarily use another server."
	case 157:
		return "Server moved - The Client should permanently use another server."
	case 159:
		return "Connection rate exceeded - The connection rate limit has been exceeded."
	}

	return ""
}
