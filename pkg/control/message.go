/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package control

import (
	"encoding/binary"
	"io"

	"github.com/google/uuid"
)

const (
	maximumSupportedVersion uint8 = 0
	outboundMessageVersion        = maximumSupportedVersion
)

type MessageFlag uint8

/*
	MessageHeader represents a message header

	+---------+---------+-------+----------+--------+
	|         |   0-8   |  8-16 |   16-24  |  24-32 |
	+---------+---------+-------+----------+--------+
	| 0-32    | version | flags | <unused> | opcode |
	+---------+---------+-------+----------+--------+
	| 32-64   |              uuid[0:4]              |
	+---------+-------------------------------------+
	| 64-96   |              uuid[4:8]              |
	+---------+-------------------------------------+
	| 96-128  |              uuid[8:12]             |
	+---------+-------------------------------------+
	| 128-160 |             uuid[12:16]             |
	+---------+-------------------------------------+
	| 160-192 |                length               |
	+---------+-------------------------------------+
*/
type MessageHeader struct {
	version uint8
	flags   uint8
	opcode  uint8
	uuid    [16]byte
	// In bytes
	length uint32
}

func (m MessageHeader) Version() uint8 {
	return m.version
}

func (m MessageHeader) Check(flag MessageFlag) bool {
	return (m.flags & uint8(flag)) == uint8(flag)
}

func (m MessageHeader) OpCode() uint8 {
	return m.opcode
}

func (m MessageHeader) UUID() uuid.UUID {
	return m.uuid
}

func (m MessageHeader) Length() uint32 {
	return m.length
}

func (m MessageHeader) WriteTo(w io.Writer) (int64, error) {
	var b [4]byte
	var n int64
	b[0] = m.version
	b[1] = m.flags
	b[3] = m.opcode
	n1, err := w.Write(b[0:4])
	n = n + int64(n1)
	if err != nil {
		return n, err
	}

	n1, err = w.Write(m.uuid[0:16])
	n = n + int64(n1)
	if err != nil {
		return n, err
	}

	binary.BigEndian.PutUint32(b[0:4], m.length)
	n1, err = w.Write(b[0:4])
	n = n + int64(n1)
	return n, err
}

func messageHeaderFromBytes(b [24]byte) MessageHeader {
	m := MessageHeader{}
	m.version = b[0]
	m.flags = b[1]
	m.opcode = b[3]
	for i := 0; i < 16; i++ {
		m.uuid[i] = b[4+i]
	}
	m.length = binary.BigEndian.Uint32(b[20:24])
	return m
}

type InboundMessage struct {
	MessageHeader
	Payload []byte
}

func (msg *InboundMessage) ReadFrom(r io.Reader) (count int64, err error) {
	var b [24]byte
	var n int
	n, err = io.ReadAtLeast(io.LimitReader(r, 24), b[0:24], 24)
	count = count + int64(n)
	if err != nil {
		return count, err
	}

	msg.MessageHeader = messageHeaderFromBytes(b)
	if msg.Length() != 0 {
		// We need to read the payload
		msg.Payload = make([]byte, msg.Length())
		n, err = io.ReadAtLeast(io.LimitReader(r, int64(msg.Length())), msg.Payload, int(msg.Length()))
		count = count + int64(n)
	}
	return count, err
}

type OutboundMessage struct {
	MessageHeader
	payload []byte
}

func (msg *OutboundMessage) WriteTo(w io.Writer) (count int64, err error) {
	n, err := msg.MessageHeader.WriteTo(w)
	count = count + n
	if err != nil {
		return count, err
	}

	if msg.payload != nil {
		var n1 int
		n1, err = w.Write(msg.payload)
		count = count + int64(n1)
	}
	return count, err
}

func NewOutboundMessage(opcode uint8, payload []byte) OutboundMessage {
	return OutboundMessage{
		MessageHeader: MessageHeader{
			version: outboundMessageVersion,
			flags:   0,
			opcode:  opcode,
			uuid:    uuid.New(),
			length:  uint32(len(payload)),
		},
		payload: payload,
	}
}

func NewOutboundMessageWithUUID(uuid [16]byte, opcode uint8, payload []byte) OutboundMessage {
	return OutboundMessage{
		MessageHeader: MessageHeader{
			version: outboundMessageVersion,
			flags:   0,
			opcode:  opcode,
			uuid:    uuid,
			length:  uint32(len(payload)),
		},
		payload: payload,
	}
}
