package packets

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
)

// Puback is the Variable Header definition for a Puback control packet
type Puback struct {
	Properties *Properties
	PacketID   uint16
	ReasonCode byte
}

// PubackSuccess, etc are the list of valid puback reason codes.
const (
	PubackSuccess                     = 0x00
	PubackNoMatchingSubscribers       = 0x10
	PubackUnspecifiedError            = 0x80
	PubackImplementationSpecificError = 0x83
	PubackNotAuthorized               = 0x87
	PubackTopicNameInvalid            = 0x90
	PubackPacketIdentifierInUse       = 0x91
	PubackQuotaExceeded               = 0x97
	PubackPayloadFormatInvalid        = 0x99
)

func (p *Puback) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "PUBACK: PacketID:%d ReasonCode:%X", p.PacketID, p.ReasonCode)
	if p.Properties != nil {
		fmt.Fprintf(&b, " Properties:\n%s", p.Properties)
	} else {
		fmt.Fprint(&b, "\n")
	}

	return b.String()
}

//Unpack is the implementation of the interface required function for a packet
func (p *Puback) Unpack(r *bytes.Buffer) error {
	var err error
	success := r.Len() == 2
	noProps := r.Len() == 3
	p.PacketID, err = readUint16(r)
	if err != nil {
		return err
	}
	if !success {
		p.ReasonCode, err = r.ReadByte()
		if err != nil {
			return err
		}

		if !noProps {
			err = p.Properties.Unpack(r, PUBACK)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Buffers is the implementation of the interface required function for a packet
func (p *Puback) Buffers() net.Buffers {
	var b bytes.Buffer
	writeUint16(p.PacketID, &b)
	b.WriteByte(p.ReasonCode)
	idvp := p.Properties.Pack(PUBACK)
	propLen := encodeVBI(len(idvp))
	n := net.Buffers{b.Bytes(), propLen}
	if len(idvp) > 0 {
		n = append(n, idvp)
	}
	return n
}

// WriteTo is the implementation of the interface required function for a packet
func (p *Puback) WriteTo(w io.Writer) (int64, error) {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: PUBACK}}
	cp.Content = p

	return cp.WriteTo(w)
}

// Reason returns a string representation of the meaning of the ReasonCode
func (p *Puback) Reason() string {
	switch p.ReasonCode {
	case 0:
		return "The message is accepted. Publication of the QoS 1 message proceeds."
	case 16:
		return "The message is accepted but there are no subscribers. This is sent only by the Server. If the Server knows that there are no matching subscribers, it MAY use this Reason Code instead of 0x00 (Success)."
	case 128:
		return "The receiver does not accept the publish but either does not want to reveal the reason, or it does not match one of the other values."
	case 131:
		return "The PUBLISH is valid but the receiver is not willing to accept it."
	case 135:
		return "The PUBLISH is not authorized."
	case 144:
		return "The Topic Name is not malformed, but is not accepted by this Client or Server."
	case 145:
		return "The Packet Identifier is already in use. This might indicate a mismatch in the Session State between the Client and Server."
	case 151:
		return "An implementation or administrative imposed limit has been exceeded."
	case 153:
		return "The payload format does not match the specified Payload Format Indicator."
	}

	return ""
}
