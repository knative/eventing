package packets

import (
	"bytes"
	"fmt"
	"io"
	"net"
)

// Unsubscribe is the Variable Header definition for a Unsubscribe control packet
type Unsubscribe struct {
	Topics     []string
	Properties *Properties
	PacketID   uint16
}

func (u *Unsubscribe) String() string {
	return fmt.Sprintf("UNSUBSCRIBE: PacketID:%d Topics:%v Properties:\n%s", u.PacketID, u.Topics, u.Properties)
}

// SetIdentifier sets the packet identifier
func (u *Unsubscribe) SetIdentifier(packetID uint16) {
	u.PacketID = packetID
}

// Type returns the current packet type
func (s *Unsubscribe) Type() byte {
	return UNSUBSCRIBE
}

// Unpack is the implementation of the interface required function for a packet
func (u *Unsubscribe) Unpack(r *bytes.Buffer) error {
	var err error
	u.PacketID, err = readUint16(r)
	if err != nil {
		return err
	}

	err = u.Properties.Unpack(r, UNSUBSCRIBE)
	if err != nil {
		return err
	}

	for {
		t, err := readString(r)
		if err != nil && err != io.EOF {
			return err
		}
		if err == io.EOF {
			break
		}
		u.Topics = append(u.Topics, t)
	}

	return nil
}

// Buffers is the implementation of the interface required function for a packet
func (u *Unsubscribe) Buffers() net.Buffers {
	var b bytes.Buffer
	writeUint16(u.PacketID, &b)
	var topics bytes.Buffer
	for _, t := range u.Topics {
		writeString(t, &topics)
	}
	idvp := u.Properties.Pack(UNSUBSCRIBE)
	propLen := encodeVBI(len(idvp))
	return net.Buffers{b.Bytes(), propLen, idvp, topics.Bytes()}
}

// WriteTo is the implementation of the interface required function for a packet
func (u *Unsubscribe) WriteTo(w io.Writer) (int64, error) {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: UNSUBSCRIBE, Flags: 2}}
	cp.Content = u

	return cp.WriteTo(w)
}
