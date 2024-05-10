package paho

import (
	"fmt"
	"strings"

	"github.com/eclipse/paho.golang/packets"
)

type (
	// Connack is a representation of the MQTT Connack packet
	Connack struct {
		Properties     *ConnackProperties
		ReasonCode     byte
		SessionPresent bool
	}

	// ConnackProperties is a struct of the properties that can be set
	// for a Connack packet
	ConnackProperties struct {
		SessionExpiryInterval *uint32
		AuthData              []byte
		AuthMethod            string
		ResponseInfo          string
		ServerReference       string
		ReasonString          string
		AssignedClientID      string
		MaximumPacketSize     *uint32
		ReceiveMaximum        *uint16
		TopicAliasMaximum     *uint16
		ServerKeepAlive       *uint16
		MaximumQoS            *byte
		User                  UserProperties
		WildcardSubAvailable  bool
		SubIDAvailable        bool
		SharedSubAvailable    bool
		RetainAvailable       bool
	}
)

// InitProperties is a function that takes a lower level
// Properties struct and completes the properties of the Connack on
// which it is called
func (c *Connack) InitProperties(p *packets.Properties) {
	c.Properties = &ConnackProperties{
		AssignedClientID:      p.AssignedClientID,
		ServerKeepAlive:       p.ServerKeepAlive,
		WildcardSubAvailable:  true,
		SubIDAvailable:        true,
		SharedSubAvailable:    true,
		RetainAvailable:       true,
		ResponseInfo:          p.ResponseInfo,
		SessionExpiryInterval: p.SessionExpiryInterval,
		AuthMethod:            p.AuthMethod,
		AuthData:              p.AuthData,
		ServerReference:       p.ServerReference,
		ReasonString:          p.ReasonString,
		ReceiveMaximum:        p.ReceiveMaximum,
		TopicAliasMaximum:     p.TopicAliasMaximum,
		MaximumQoS:            p.MaximumQOS,
		MaximumPacketSize:     p.MaximumPacketSize,
		User:                  UserPropertiesFromPacketUser(p.User),
	}

	if p.WildcardSubAvailable != nil {
		c.Properties.WildcardSubAvailable = *p.WildcardSubAvailable == 1
	}
	if p.SubIDAvailable != nil {
		c.Properties.SubIDAvailable = *p.SubIDAvailable == 1
	}
	if p.SharedSubAvailable != nil {
		c.Properties.SharedSubAvailable = *p.SharedSubAvailable == 1
	}
	if p.RetainAvailable != nil {
		c.Properties.RetainAvailable = *p.RetainAvailable == 1
	}
}

// ConnackFromPacketConnack takes a packets library Connack and
// returns a paho library Connack
func ConnackFromPacketConnack(c *packets.Connack) *Connack {
	v := &Connack{
		SessionPresent: c.SessionPresent,
		ReasonCode:     c.ReasonCode,
	}
	v.InitProperties(c.Properties)

	return v
}

// String implement fmt.Stringer (mainly to simplify debugging)
func (c *Connack) String() string {
	return fmt.Sprintf("CONNACK: ReasonCode:%d SessionPresent:%t\nProperties:\n%s", c.ReasonCode, c.SessionPresent, c.Properties)
}

// String implement fmt.Stringer (mainly to simplify debugging)
func (p *ConnackProperties) String() string {
	var b strings.Builder
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
	fmt.Fprintf(&b, "\tRetainAvailable:%t\n", p.RetainAvailable)
	if p.MaximumPacketSize != nil {
		fmt.Fprintf(&b, "\tMaximumPacketSize:%d\n", *p.MaximumPacketSize)
	}
	fmt.Fprintf(&b, "\tWildcardSubAvailable:%t\n", p.WildcardSubAvailable)
	fmt.Fprintf(&b, "\tSubIDAvailable:%t\n", p.SubIDAvailable)
	fmt.Fprintf(&b, "\tSharedSubAvailable:%t\n", p.SharedSubAvailable)

	if len(p.User) > 0 {
		fmt.Fprint(&b, "\tUser Properties:\n")
		for _, v := range p.User {
			fmt.Fprintf(&b, "\t\t%s:%s\n", v.Key, v.Value)
		}
	}

	return b.String()
}
