package packets

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
)

// PacketType is a type alias to byte representing the different
// MQTT control packet types
// type PacketType byte

// The following consts are the packet type number for each of the
// different control packets in MQTT
const (
	_ byte = iota
	CONNECT
	CONNACK
	PUBLISH
	PUBACK
	PUBREC
	PUBREL
	PUBCOMP
	SUBSCRIBE
	SUBACK
	UNSUBSCRIBE
	UNSUBACK
	PINGREQ
	PINGRESP
	DISCONNECT
	AUTH
)

type (
	// Packet is the interface defining the unique parts of a controlpacket
	Packet interface {
		Unpack(*bytes.Buffer) error
		Buffers() net.Buffers
		WriteTo(io.Writer) (int64, error)
	}

	// FixedHeader is the definition of a control packet fixed header
	FixedHeader struct {
		remainingLength int
		Type            byte
		Flags           byte
	}

	// ControlPacket is the definition of a control packet
	ControlPacket struct {
		Content Packet
		FixedHeader
	}
)

// NewThreadSafeConn wraps net.Conn with a mutex. ControlPacket uses it in
// WriteTo method to ensure parallel writes are thread-Safe.
func NewThreadSafeConn(c net.Conn) net.Conn {
	type threadSafeConn struct {
		net.Conn
		sync.Locker
	}

	return &threadSafeConn{
		Conn:   c,
		Locker: &sync.Mutex{},
	}
}

// WriteTo operates on a FixedHeader and takes the option values and produces
// the wire format byte that represents these.
func (f *FixedHeader) WriteTo(w io.Writer) (int64, error) {
	if _, err := w.Write([]byte{byte(f.Type)<<4 | f.Flags}); err != nil {
		return 0, err
	}
	if _, err := w.Write(encodeVBI(f.remainingLength)); err != nil {
		return 0, err
	}

	return 0, nil
}

// PacketID is a helper function that returns the value of the PacketID
// field from any kind of mqtt packet in the Content element
func (c *ControlPacket) PacketID() uint16 {
	switch r := c.Content.(type) {
	case *Publish:
		return r.PacketID
	case *Puback:
		return r.PacketID
	case *Pubrec:
		return r.PacketID
	case *Pubrel:
		return r.PacketID
	case *Pubcomp:
		return r.PacketID
	case *Subscribe:
		return r.PacketID
	case *Suback:
		return r.PacketID
	case *Unsubscribe:
		return r.PacketID
	case *Unsuback:
		return r.PacketID
	default:
		return 0
	}
}

// PacketType returns the packet type as a string
func (c *ControlPacket) PacketType() string {
	return [...]string{
		"",
		"CONNECT",
		"CONNACK",
		"PUBLISH",
		"PUBACK",
		"PUBREC",
		"PUBREL",
		"PUBCOMP",
		"SUBSCRIBE",
		"SUBACK",
		"UNSUBSCRIBE",
		"UNSUBACK",
		"PINGREQ",
		"PINGRESP",
		"DISCONNECT",
		"AUTH",
	}[c.FixedHeader.Type]
}

// String implements fmt.Stringer (mainly for debugging purposes)
func (c *ControlPacket) String() string {
	switch p := c.Content.(type) {
	case *Connect:
		return p.String()
	case *Connack:
		return p.String()
	case *Publish:
		return p.String()
	case *Puback:
		return p.String()
	case *Pubrec:
		return p.String()
	case *Pubrel:
		return p.String()
	case *Pubcomp:
		return p.String()
	case *Subscribe:
		return p.String()
	case *Suback:
		return p.String()
	case *Unsubscribe:
		return p.String()
	case *Unsuback:
		return p.String()
	case *Pingreq:
		return p.String()
	case *Pingresp:
		return p.String()
	case *Disconnect:
		return p.String()
	case *Auth:
		return p.String()
	default:
		return fmt.Sprintf("Unknown packet type: %d", c.Type)
	}
}

// NewControlPacket takes a packetType and returns a pointer to a
// ControlPacket where the VariableHeader field is a pointer to an
// instance of a VariableHeader definition for that packetType
func NewControlPacket(t byte) *ControlPacket {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: t}}
	switch t {
	case CONNECT:
		cp.Content = &Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			Properties:      &Properties{},
		}
	case CONNACK:
		cp.Content = &Connack{Properties: &Properties{}}
	case PUBLISH:
		cp.Content = &Publish{Properties: &Properties{}}
	case PUBACK:
		cp.Content = &Puback{Properties: &Properties{}}
	case PUBREC:
		cp.Content = &Pubrec{Properties: &Properties{}}
	case PUBREL:
		cp.Flags = 2
		cp.Content = &Pubrel{Properties: &Properties{}}
	case PUBCOMP:
		cp.Content = &Pubcomp{Properties: &Properties{}}
	case SUBSCRIBE:
		cp.Flags = 2
		cp.Content = &Subscribe{Properties: &Properties{}}
	case SUBACK:
		cp.Content = &Suback{Properties: &Properties{}}
	case UNSUBSCRIBE:
		cp.Flags = 2
		cp.Content = &Unsubscribe{Properties: &Properties{}}
	case UNSUBACK:
		cp.Content = &Unsuback{Properties: &Properties{}}
	case PINGREQ:
		cp.Content = &Pingreq{}
	case PINGRESP:
		cp.Content = &Pingresp{}
	case DISCONNECT:
		cp.Content = &Disconnect{Properties: &Properties{}}
	case AUTH:
		cp.Flags = 1
		cp.Content = &Auth{Properties: &Properties{}}
	default:
		return nil
	}
	return cp
}

// ReadPacket reads a control packet from a io.Reader and returns a completed
// struct with the appropriate data
func ReadPacket(r io.Reader) (*ControlPacket, error) {
	t := [1]byte{}
	_, err := io.ReadFull(r, t[:])
	if err != nil {
		return nil, err
	}
	// cp := NewControlPacket(PacketType(t[0] >> 4))
	// if cp == nil {
	// 	return nil, fmt.Errorf("invalid packet type requested, %d", t[0]>>4)
	// }

	pt := t[0] >> 4
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: pt}}
	switch pt {
	case CONNECT:
		cp.Content = &Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			Properties:      &Properties{},
		}
	case CONNACK:
		cp.Content = &Connack{Properties: &Properties{}}
	case PUBLISH:
		cp.Content = &Publish{Properties: &Properties{}}
	case PUBACK:
		cp.Content = &Puback{Properties: &Properties{}}
	case PUBREC:
		cp.Content = &Pubrec{Properties: &Properties{}}
	case PUBREL:
		cp.Flags = 2
		cp.Content = &Pubrel{Properties: &Properties{}}
	case PUBCOMP:
		cp.Content = &Pubcomp{Properties: &Properties{}}
	case SUBSCRIBE:
		cp.Flags = 2
		cp.Content = &Subscribe{Properties: &Properties{}}
	case SUBACK:
		cp.Content = &Suback{Properties: &Properties{}}
	case UNSUBSCRIBE:
		cp.Flags = 2
		cp.Content = &Unsubscribe{Properties: &Properties{}}
	case UNSUBACK:
		cp.Content = &Unsuback{Properties: &Properties{}}
	case PINGREQ:
		cp.Content = &Pingreq{}
	case PINGRESP:
		cp.Content = &Pingresp{}
	case DISCONNECT:
		cp.Content = &Disconnect{Properties: &Properties{}}
	case AUTH:
		cp.Flags = 1
		cp.Content = &Auth{Properties: &Properties{}}
	default:
		return nil, fmt.Errorf("unknown packet type %d requested", pt)
	}

	cp.Flags = t[0] & 0xF
	if cp.Type == PUBLISH { // Publish is the only packet with flags in the fixed header
		cp.Content.(*Publish).QoS = (cp.Flags >> 1) & 0x3
		cp.Content.(*Publish).Duplicate = cp.Flags&(1<<3) != 0
		cp.Content.(*Publish).Retain = cp.Flags&1 != 0
	}
	vbi, err := getVBI(r)
	if err != nil {
		return nil, err
	}
	cp.remainingLength, err = decodeVBI(vbi)
	if err != nil {
		return nil, err
	}

	var content bytes.Buffer
	content.Grow(cp.remainingLength)

	n, err := io.CopyN(&content, r, int64(cp.remainingLength))
	if err != nil {
		return nil, err
	}

	if n != int64(cp.remainingLength) {
		return nil, fmt.Errorf("failed to read packet, expected %d bytes, read %d", cp.remainingLength, n)
	}
	err = cp.Content.Unpack(&content)
	if err != nil {
		return nil, err
	}
	return cp, nil
}

// WriteTo writes a packet to an io.Writer, handling packing all the parts of
// a control packet.
func (c *ControlPacket) WriteTo(w io.Writer) (int64, error) {
	c.remainingLength = 0 // ignore previous remainingLength (if any)
	buffers := c.Content.Buffers()
	for _, b := range buffers {
		c.remainingLength += len(b)
	}

	if c.Type == PUBLISH { // Fixed flags for PUBLISH packets contain QOS, DUP and RETAIN flags.
		p := c.Content.(*Publish)
		f := p.QoS << 1
		if p.Duplicate {
			f |= 1 << 3
		}
		if p.Retain {
			f |= 1
		}
		c.FixedHeader.Flags = c.Type<<4 | f
	}

	var header bytes.Buffer
	if _, err := c.FixedHeader.WriteTo(&header); err != nil {
		return 0, err
	}

	buffers = append(net.Buffers{header.Bytes()}, buffers...)

	if safe, ok := w.(sync.Locker); ok {
		safe.Lock()
		defer safe.Unlock()
	}
	return buffers.WriteTo(w)
}

func encodeVBI(length int) []byte {
	var x int
	b := [4]byte{}
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		b[x] = digit
		x++
		if length == 0 {
			return b[:x]
		}
	}
}

func encodeVBIdirect(length int, buf *bytes.Buffer) {
	var x int
	b := [4]byte{}
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		b[x] = digit
		x++
		if length == 0 {
			buf.Write(b[:x])
			return
		}
	}
}

func getVBI(r io.Reader) (*bytes.Buffer, error) {
	var ret bytes.Buffer
	digit := [1]byte{}
	for {
		_, err := io.ReadFull(r, digit[:])
		if err != nil {
			return nil, err
		}
		ret.WriteByte(digit[0])
		if digit[0] <= 0x7f {
			return &ret, nil
		}
	}
}

func decodeVBI(r *bytes.Buffer) (int, error) {
	var vbi uint32
	var multiplier uint32
	for {
		digit, err := r.ReadByte()
		if err != nil && err != io.EOF {
			return 0, err
		}
		vbi |= uint32(digit&127) << multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7
	}
	return int(vbi), nil
}

func writeUint16(u uint16, b *bytes.Buffer) error {
	if err := b.WriteByte(byte(u >> 8)); err != nil {
		return err
	}
	return b.WriteByte(byte(u))
}

func writeUint32(u uint32, b *bytes.Buffer) error {
	if err := b.WriteByte(byte(u >> 24)); err != nil {
		return err
	}
	if err := b.WriteByte(byte(u >> 16)); err != nil {
		return err
	}
	if err := b.WriteByte(byte(u >> 8)); err != nil {
		return err
	}
	return b.WriteByte(byte(u))
}

func writeString(s string, b *bytes.Buffer) {
	writeUint16(uint16(len(s)), b)
	b.WriteString(s)
}

func writeBinary(d []byte, b *bytes.Buffer) {
	writeUint16(uint16(len(d)), b)
	b.Write(d)
}

func readUint16(b *bytes.Buffer) (uint16, error) {
	b1, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b2, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	return (uint16(b1) << 8) | uint16(b2), nil
}

func readUint32(b *bytes.Buffer) (uint32, error) {
	b1, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b2, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b3, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b4, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	return (uint32(b1) << 24) | (uint32(b2) << 16) | (uint32(b3) << 8) | uint32(b4), nil
}

func readBinary(b *bytes.Buffer) ([]byte, error) {
	size, err := readUint16(b)
	if err != nil {
		return nil, err
	}

	var s bytes.Buffer
	s.Grow(int(size))
	if _, err := io.CopyN(&s, b, int64(size)); err != nil {
		return nil, err
	}

	return s.Bytes(), nil
}

func readString(b *bytes.Buffer) (string, error) {
	s, err := readBinary(b)
	return string(s), err
}
