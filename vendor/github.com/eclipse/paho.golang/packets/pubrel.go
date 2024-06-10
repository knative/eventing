package packets

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
)

// Pubrel is the Variable Header definition for a Pubrel control packet
type Pubrel struct {
	Properties *Properties
	PacketID   uint16
	ReasonCode byte
}

func (p *Pubrel) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "PUBREL: ReasonCode:%X PacketID:%d", p.ReasonCode, p.PacketID)
	if p.Properties != nil {
		fmt.Fprintf(&b, " Properties:\n%s", p.Properties)
	} else {
		fmt.Fprint(&b, "\n")
	}

	return b.String()
}

//Unpack is the implementation of the interface required function for a packet
func (p *Pubrel) Unpack(r *bytes.Buffer) error {
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
func (p *Pubrel) Buffers() net.Buffers {
	var b bytes.Buffer
	writeUint16(p.PacketID, &b)
	b.WriteByte(p.ReasonCode)
	n := net.Buffers{b.Bytes()}
	idvp := p.Properties.Pack(PUBREL)
	propLen := encodeVBI(len(idvp))
	if len(idvp) > 0 {
		n = append(n, propLen)
		n = append(n, idvp)
	}
	return n
}

// WriteTo is the implementation of the interface required function for a packet
func (p *Pubrel) WriteTo(w io.Writer) (int64, error) {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: PUBREL, Flags: 2}}
	cp.Content = p

	return cp.WriteTo(w)
}
