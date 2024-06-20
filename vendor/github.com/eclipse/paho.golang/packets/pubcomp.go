package packets

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
)

// Pubcomp is the Variable Header definition for a Pubcomp control packet
type Pubcomp struct {
	Properties *Properties
	PacketID   uint16
	ReasonCode byte
}

// PubcompSuccess, etc are the list of valid pubcomp reason codes.
const (
	PubcompSuccess                  = 0x00
	PubcompPacketIdentifierNotFound = 0x92
)

func (p *Pubcomp) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "PUBCOMP: ReasonCode:%X PacketID:%d", p.ReasonCode, p.PacketID)
	if p.Properties != nil {
		fmt.Fprintf(&b, " Properties:\n%s", p.Properties)
	} else {
		fmt.Fprint(&b, "\n")
	}

	return b.String()
}

//Unpack is the implementation of the interface required function for a packet
func (p *Pubcomp) Unpack(r *bytes.Buffer) error {
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
func (p *Pubcomp) Buffers() net.Buffers {
	var b bytes.Buffer
	writeUint16(p.PacketID, &b)
	b.WriteByte(p.ReasonCode)
	n := net.Buffers{b.Bytes()}
	idvp := p.Properties.Pack(PUBCOMP)
	propLen := encodeVBI(len(idvp))
	if len(idvp) > 0 {
		n = append(n, propLen)
		n = append(n, idvp)
	}
	return n
}

// WriteTo is the implementation of the interface required function for a packet
func (p *Pubcomp) WriteTo(w io.Writer) (int64, error) {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: PUBCOMP}}
	cp.Content = p

	return cp.WriteTo(w)
}

// Reason returns a string representation of the meaning of the ReasonCode
func (p *Pubcomp) Reason() string {
	switch p.ReasonCode {
	case 0:
		return "Success - Packet Identifier released. Publication of QoS 2 message is complete."
	case 146:
		return "Packet Identifier not found - The Packet Identifier is not known. This is not an error during recovery, but at other times indicates a mismatch between the Session State on the Client and Server."
	}

	return ""
}
