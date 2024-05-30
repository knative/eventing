package paho

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/packets"
	"golang.org/x/sync/semaphore"
)

type MQTTVersion byte

const (
	MQTTv311 MQTTVersion = 4
	MQTTv5   MQTTVersion = 5
)

const defaultSendAckInterval = 50 * time.Millisecond

var (
	ErrManualAcknowledgmentDisabled = errors.New("manual acknowledgments disabled")
)

type (
	// ClientConfig are the user configurable options for the client, an
	// instance of this struct is passed into NewClient(), not all options
	// are required to be set, defaults are provided for Persistence, MIDs,
	// PingHandler, PacketTimeout and Router.
	ClientConfig struct {
		ClientID string
		// Conn is the connection to broker.
		// BEWARE that most wrapped net.Conn implementations like tls.Conn are
		// not thread safe for writing. To fix, use packets.NewThreadSafeConn
		// wrapper or extend the custom net.Conn struct with sync.Locker.
		Conn          net.Conn
		MIDs          MIDService
		AuthHandler   Auther
		PingHandler   Pinger
		Router        Router
		Persistence   Persistence
		PacketTimeout time.Duration
		// OnServerDisconnect is called only when a packets.DISCONNECT is received from server
		OnServerDisconnect func(*Disconnect)
		// OnClientError is for example called on net.Error
		OnClientError func(error)
		// PublishHook allows a user provided function to be called before
		// a Publish packet is sent allowing it to inspect or modify the
		// Publish, an example of the utility of this is provided in the
		// Topic Alias Handler extension which will automatically assign
		// and use topic alias values rather than topic strings.
		PublishHook func(*Publish)
		// EnableManualAcknowledgment is used to control the acknowledgment of packets manually.
		// BEWARE that the MQTT specs require clients to send acknowledgments in the order in which the corresponding
		// PUBLISH packets were received.
		// Consider the following scenario: the client receives packets 1,2,3,4
		// If you acknowledge 3 first, no ack is actually sent to the server but it's buffered until also 1 and 2
		// are acknowledged.
		EnableManualAcknowledgment bool
		// SendAcksInterval is used only when EnableManualAcknowledgment is true
		// it determines how often the client tries to send a batch of acknowledgments in the right order to the server.
		SendAcksInterval time.Duration
	}
	// Client is the struct representing an MQTT client
	Client struct {
		mu sync.Mutex
		ClientConfig
		// raCtx is used for handling the MQTTv5 authentication exchange.
		raCtx          *CPContext
		stop           chan struct{}
		publishPackets chan *packets.Publish
		acksTracker    acksTracker
		workers        sync.WaitGroup
		serverProps    CommsProperties
		clientProps    CommsProperties
		serverInflight *semaphore.Weighted
		clientInflight *semaphore.Weighted
		debug          Logger
		errors         Logger
	}

	// CommsProperties is a struct of the communication properties that may
	// be set by the server in the Connack and that the client needs to be
	// aware of for future subscribes/publishes
	CommsProperties struct {
		MaximumPacketSize    uint32
		ReceiveMaximum       uint16
		TopicAliasMaximum    uint16
		MaximumQoS           byte
		RetainAvailable      bool
		WildcardSubAvailable bool
		SubIDAvailable       bool
		SharedSubAvailable   bool
	}

	caContext struct {
		Context context.Context
		Return  chan *packets.Connack
	}
)

// NewClient is used to create a new default instance of an MQTT client.
// It returns a pointer to the new client instance.
// The default client uses the provided PingHandler, MessageID and
// StandardRouter implementations, and a noop Persistence.
// These should be replaced if desired before the client is connected.
// client.Conn *MUST* be set to an already connected net.Conn before
// Connect() is called.
func NewClient(conf ClientConfig) *Client {
	c := &Client{
		serverProps: CommsProperties{
			ReceiveMaximum:       65535,
			MaximumQoS:           2,
			MaximumPacketSize:    0,
			TopicAliasMaximum:    0,
			RetainAvailable:      true,
			WildcardSubAvailable: true,
			SubIDAvailable:       true,
			SharedSubAvailable:   true,
		},
		clientProps: CommsProperties{
			ReceiveMaximum:    65535,
			MaximumQoS:        2,
			MaximumPacketSize: 0,
			TopicAliasMaximum: 0,
		},
		ClientConfig: conf,
		errors:       NOOPLogger{},
		debug:        NOOPLogger{},
	}

	if c.Persistence == nil {
		c.Persistence = &noopPersistence{}
	}
	if c.MIDs == nil {
		c.MIDs = &MIDs{index: make([]*CPContext, int(midMax))}
	}
	if c.PacketTimeout == 0 {
		c.PacketTimeout = 10 * time.Second
	}
	if c.Router == nil {
		c.Router = NewStandardRouter()
	}
	if c.PingHandler == nil {
		c.PingHandler = DefaultPingerWithCustomFailHandler(func(e error) {
			go c.error(e)
		})
	}
	if c.OnClientError == nil {
		c.OnClientError = func(e error) {}
	}

	return c
}

// Connect is used to connect the client to a server. It presumes that
// the Client instance already has a working network connection.
// The function takes a pre-prepared Connect packet, and uses that to
// establish an MQTT connection. Assuming the connection completes
// successfully the rest of the client is initiated and the Connack
// returned. Otherwise the failure Connack (if there is one) is returned
// along with an error indicating the reason for the failure to connect.
func (c *Client) Connect(ctx context.Context, cp *Connect) (*Connack, error) {
	if c.Conn == nil {
		return nil, fmt.Errorf("client connection is nil")
	}

	cleanup := func() {
		close(c.stop)
		close(c.publishPackets)
		_ = c.Conn.Close()
		c.mu.Unlock()
	}

	c.mu.Lock()
	c.stop = make(chan struct{})

	var publishPacketsSize uint16 = math.MaxUint16
	if cp.Properties != nil && cp.Properties.ReceiveMaximum != nil {
		publishPacketsSize = *cp.Properties.ReceiveMaximum
	}
	c.publishPackets = make(chan *packets.Publish, publishPacketsSize)

	keepalive := cp.KeepAlive
	c.ClientID = cp.ClientID
	if cp.Properties != nil {
		if cp.Properties.MaximumPacketSize != nil {
			c.clientProps.MaximumPacketSize = *cp.Properties.MaximumPacketSize
		}
		if cp.Properties.MaximumQOS != nil {
			c.clientProps.MaximumQoS = *cp.Properties.MaximumQOS
		}
		if cp.Properties.ReceiveMaximum != nil {
			c.clientProps.ReceiveMaximum = *cp.Properties.ReceiveMaximum
		}
		if cp.Properties.TopicAliasMaximum != nil {
			c.clientProps.TopicAliasMaximum = *cp.Properties.TopicAliasMaximum
		}
	}

	c.debug.Println("connecting")
	connCtx, cf := context.WithTimeout(ctx, c.PacketTimeout)
	defer cf()

	ccp := cp.Packet()
	ccp.ProtocolName = "MQTT"
	ccp.ProtocolVersion = 5

	c.debug.Println("sending CONNECT")
	if _, err := ccp.WriteTo(c.Conn); err != nil {
		cleanup()
		return nil, err
	}

	c.debug.Println("waiting for CONNACK/AUTH")
	var (
		caPacket    *packets.Connack
		// We use buffered channels to prevent goroutine leak. The Details are below.
		// - c.expectConnack waits to send data to caPacketCh or caPacketErr.
		// - If connCtx is cancelled (done) before c.expectConnack finishes to send data to either "unbuffered" channel,
		//   c.expectConnack cannot exit (goroutine leak).
		caPacketCh  = make(chan *packets.Connack, 1)
		caPacketErr = make(chan error, 1)
	)
	go c.expectConnack(caPacketCh, caPacketErr)
	select {
	case <-connCtx.Done():
		if ctxErr := connCtx.Err(); ctxErr != nil {
			c.debug.Println(fmt.Sprintf("terminated due to context: %v", ctxErr))
		}
		cleanup()
		return nil, connCtx.Err()
	case err := <-caPacketErr:
		c.debug.Println(err)
		cleanup()
		return nil, err
	case caPacket = <-caPacketCh:
	}

	ca := ConnackFromPacketConnack(caPacket)

	if ca.ReasonCode >= 0x80 {
		var reason string
		c.debug.Println("received an error code in Connack:", ca.ReasonCode)
		if ca.Properties != nil {
			reason = ca.Properties.ReasonString
		}
		cleanup()
		return ca, fmt.Errorf("failed to connect to server: %s", reason)
	}

	// no more possible calls to cleanup(), defer an unlock
	defer c.mu.Unlock()

	if ca.Properties != nil {
		if ca.Properties.ServerKeepAlive != nil {
			keepalive = *ca.Properties.ServerKeepAlive
		}
		if ca.Properties.AssignedClientID != "" {
			c.ClientID = ca.Properties.AssignedClientID
		}
		if ca.Properties.ReceiveMaximum != nil {
			c.serverProps.ReceiveMaximum = *ca.Properties.ReceiveMaximum
		}
		if ca.Properties.MaximumQoS != nil {
			c.serverProps.MaximumQoS = *ca.Properties.MaximumQoS
		}
		if ca.Properties.MaximumPacketSize != nil {
			c.serverProps.MaximumPacketSize = *ca.Properties.MaximumPacketSize
		}
		if ca.Properties.TopicAliasMaximum != nil {
			c.serverProps.TopicAliasMaximum = *ca.Properties.TopicAliasMaximum
		}
		c.serverProps.RetainAvailable = ca.Properties.RetainAvailable
		c.serverProps.WildcardSubAvailable = ca.Properties.WildcardSubAvailable
		c.serverProps.SubIDAvailable = ca.Properties.SubIDAvailable
		c.serverProps.SharedSubAvailable = ca.Properties.SharedSubAvailable
	}

	c.serverInflight = semaphore.NewWeighted(int64(c.serverProps.ReceiveMaximum))
	c.clientInflight = semaphore.NewWeighted(int64(c.clientProps.ReceiveMaximum))

	c.debug.Println("received CONNACK, starting PingHandler")
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		defer c.debug.Println("returning from ping handler worker")
		c.PingHandler.Start(c.Conn, time.Duration(keepalive)*time.Second)
	}()

	c.debug.Println("starting publish packets loop")
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		defer c.debug.Println("returning from publish packets loop worker")
		c.routePublishPackets()
	}()

	c.debug.Println("starting incoming")
	c.workers.Add(1)
	go func() {
		defer c.workers.Done()
		defer c.debug.Println("returning from incoming worker")
		c.incoming()
	}()

	if c.EnableManualAcknowledgment {
		c.debug.Println("starting acking routine")

		c.acksTracker.reset()
		sendAcksInterval := defaultSendAckInterval
		if c.SendAcksInterval > 0 {
			sendAcksInterval = c.SendAcksInterval
		}

		c.workers.Add(1)
		go func() {
			defer c.workers.Done()
			defer c.debug.Println("returning from ack tracker routine")
			t := time.NewTicker(sendAcksInterval)
			for {
				select {
				case <-c.stop:
					return
				case <-t.C:
					c.acksTracker.flush(func(pbs []*packets.Publish) {
						for _, pb := range pbs {
							c.ack(pb)
						}
					})
				}
			}
		}()
	}

	return ca, nil
}

func (c *Client) Ack(pb *Publish) error {
	if !c.EnableManualAcknowledgment {
		return ErrManualAcknowledgmentDisabled
	}
	if pb.QoS == 0 {
		return nil
	}
	return c.acksTracker.markAsAcked(pb.Packet())
}

func (c *Client) ack(pb *packets.Publish) {
	switch pb.QoS {
	case 1:
		pa := packets.Puback{
			Properties: &packets.Properties{},
			PacketID:   pb.PacketID,
		}
		c.debug.Println("sending PUBACK")
		_, err := pa.WriteTo(c.Conn)
		if err != nil {
			c.errors.Printf("failed to send PUBACK for %d: %s", pb.PacketID, err)
		}
	case 2:
		pr := packets.Pubrec{
			Properties: &packets.Properties{},
			PacketID:   pb.PacketID,
		}
		c.debug.Printf("sending PUBREC")
		_, err := pr.WriteTo(c.Conn)
		if err != nil {
			c.errors.Printf("failed to send PUBREC for %d: %s", pb.PacketID, err)
		}
	}
}

func (c *Client) routePublishPackets() {
	for {
		select {
		case <-c.stop:
			return
		case pb, open := <-c.publishPackets:
			if !open {
				return
			}

			if !c.ClientConfig.EnableManualAcknowledgment {
				c.Router.Route(pb)
				c.ack(pb)
				continue
			}

			if pb.QoS != 0 {
				c.acksTracker.add(pb)
			}

			c.Router.Route(pb)
		}
	}
}

// incoming is the Client function that reads and handles incoming
// packets from the server. The function is started as a goroutine
// from Connect(), it exits when it receives a server initiated
// Disconnect, the Stop channel is closed or there is an error reading
// a packet from the network connection
func (c *Client) incoming() {
	defer c.debug.Println("client stopping, incoming stopping")
	for {
		select {
		case <-c.stop:
			return
		default:
			recv, err := packets.ReadPacket(c.Conn)
			if err != nil {
				go c.error(err)
				return
			}
			switch recv.Type {
			case packets.CONNACK:
				c.debug.Println("received CONNACK")
				go c.error(fmt.Errorf("received unexpected CONNACK"))
				return
			case packets.AUTH:
				c.debug.Println("received AUTH")
				ap := recv.Content.(*packets.Auth)
				switch ap.ReasonCode {
				case packets.AuthSuccess:
					if c.AuthHandler != nil {
						go c.AuthHandler.Authenticated()
					}
					if c.raCtx != nil {
						c.raCtx.Return <- *recv
					}
				case packets.AuthContinueAuthentication:
					if c.AuthHandler != nil {
						if _, err := c.AuthHandler.Authenticate(AuthFromPacketAuth(ap)).Packet().WriteTo(c.Conn); err != nil {
							go c.error(err)
							return
						}
					}
				}
			case packets.PUBLISH:
				pb := recv.Content.(*packets.Publish)
				c.debug.Printf("received QoS%d PUBLISH", pb.QoS)
				c.mu.Lock()
				select {
				case <-c.stop:
					c.mu.Unlock()
					return
				default:
					c.publishPackets <- pb
					c.mu.Unlock()
				}
			case packets.PUBACK, packets.PUBCOMP, packets.SUBACK, packets.UNSUBACK:
				c.debug.Printf("received %s packet with id %d", recv.PacketType(), recv.PacketID())
				if cpCtx := c.MIDs.Get(recv.PacketID()); cpCtx != nil {
					cpCtx.Return <- *recv
				} else {
					c.debug.Println("received a response for a message ID we don't know:", recv.PacketID())
				}
			case packets.PUBREC:
				c.debug.Println("received PUBREC for", recv.PacketID())
				if cpCtx := c.MIDs.Get(recv.PacketID()); cpCtx == nil {
					c.debug.Println("received a PUBREC for a message ID we don't know:", recv.PacketID())
					pl := packets.Pubrel{
						PacketID:   recv.Content.(*packets.Pubrec).PacketID,
						ReasonCode: 0x92,
					}
					c.debug.Println("sending PUBREL for", pl.PacketID)
					_, err := pl.WriteTo(c.Conn)
					if err != nil {
						c.errors.Printf("failed to send PUBREL for %d: %s", pl.PacketID, err)
					}
				} else {
					pr := recv.Content.(*packets.Pubrec)
					if pr.ReasonCode >= 0x80 {
						//Received a failure code, shortcut and return
						cpCtx.Return <- *recv
					} else {
						pl := packets.Pubrel{
							PacketID: pr.PacketID,
						}
						c.debug.Println("sending PUBREL for", pl.PacketID)
						_, err := pl.WriteTo(c.Conn)
						if err != nil {
							c.errors.Printf("failed to send PUBREL for %d: %s", pl.PacketID, err)
						}
					}
				}
			case packets.PUBREL:
				c.debug.Println("received PUBREL for", recv.PacketID())
				//Auto respond to pubrels unless failure code
				pr := recv.Content.(*packets.Pubrel)
				if pr.ReasonCode >= 0x80 {
					//Received a failure code, continue
					continue
				} else {
					pc := packets.Pubcomp{
						PacketID: pr.PacketID,
					}
					c.debug.Println("sending PUBCOMP for", pr.PacketID)
					_, err := pc.WriteTo(c.Conn)
					if err != nil {
						c.errors.Printf("failed to send PUBCOMP for %d: %s", pc.PacketID, err)
					}
				}
			case packets.DISCONNECT:
				c.debug.Println("received DISCONNECT")
				if c.raCtx != nil {
					c.raCtx.Return <- *recv
				}
				go func() {
					if c.OnServerDisconnect != nil {
						go c.serverDisconnect(DisconnectFromPacketDisconnect(recv.Content.(*packets.Disconnect)))
					} else {
						go c.error(fmt.Errorf("server initiated disconnect"))
					}
				}()
				return
			case packets.PINGRESP:
				c.debug.Println("received PINGRESP")
				c.PingHandler.PingResp()
			}
		}
	}
}

func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.stop:
		//already shutting down, do nothing
		return
	default:
	}

	close(c.stop)
	close(c.publishPackets)

	c.debug.Println("client stopped")
	c.PingHandler.Stop()
	c.debug.Println("ping stopped")
	_ = c.Conn.Close()
	c.debug.Println("conn closed")
	c.acksTracker.reset()
	c.debug.Println("acks tracker reset")
}

// error is called to signify that an error situation has occurred, this
// causes the client's Stop channel to be closed (if it hasn't already been)
// which results in the other client goroutines terminating.
// It also closes the client network connection.
func (c *Client) error(e error) {
	c.debug.Println("error called:", e)
	c.close()
	c.workers.Wait()
	go c.OnClientError(e)
}

func (c *Client) serverDisconnect(d *Disconnect) {
	c.close()
	c.workers.Wait()
	c.debug.Println("calling OnServerDisconnect")
	go c.OnServerDisconnect(d)
}

// Authenticate is used to initiate a reauthentication of credentials with the
// server. This function sends the initial Auth packet to start the reauthentication
// then relies on the client AuthHandler managing any further requests from the
// server until either a successful Auth packet is passed back, or a Disconnect
// is received.
func (c *Client) Authenticate(ctx context.Context, a *Auth) (*AuthResponse, error) {
	c.debug.Println("client initiated reauthentication")

	c.mu.Lock()
	if c.raCtx != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("previous authentication is still in progress")
	}
	c.raCtx = &CPContext{ctx, make(chan packets.ControlPacket, 1)}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.raCtx = nil
		c.mu.Unlock()
	}()

	c.debug.Println("sending AUTH")
	if _, err := a.Packet().WriteTo(c.Conn); err != nil {
		return nil, err
	}

	var rp packets.ControlPacket
	select {
	case <-ctx.Done():
		if ctxErr := ctx.Err(); ctxErr != nil {
			c.debug.Println(fmt.Sprintf("terminated due to context: %v", ctxErr))
			return nil, ctxErr
		}
	case rp = <-c.raCtx.Return:
	}

	switch rp.Type {
	case packets.AUTH:
		//If we've received one here it must be successful, the only way
		//to abort a reauth is a server initiated disconnect
		return AuthResponseFromPacketAuth(rp.Content.(*packets.Auth)), nil
	case packets.DISCONNECT:
		return AuthResponseFromPacketDisconnect(rp.Content.(*packets.Disconnect)), nil
	}

	return nil, fmt.Errorf("error with Auth, didn't receive Auth or Disconnect")
}

// Subscribe is used to send a Subscription request to the MQTT server.
// It is passed a pre-prepared Subscribe packet and blocks waiting for
// a response Suback, or for the timeout to fire. Any response Suback
// is returned from the function, along with any errors.
func (c *Client) Subscribe(ctx context.Context, s *Subscribe) (*Suback, error) {
	if !c.serverProps.WildcardSubAvailable {
		for _, sub := range s.Subscriptions {
			if strings.ContainsAny(sub.Topic, "#+") {
				// Using a wildcard in a subscription when not supported
				return nil, fmt.Errorf("cannot subscribe to %s, server does not support wildcards", sub.Topic)
			}
		}
	}
	if !c.serverProps.SubIDAvailable && s.Properties != nil && s.Properties.SubscriptionIdentifier != nil {
		return nil, fmt.Errorf("cannot send subscribe with subID set, server does not support subID")
	}
	if !c.serverProps.SharedSubAvailable {
		for _, sub := range s.Subscriptions {
			if strings.HasPrefix(sub.Topic, "$share") {
				return nil, fmt.Errorf("cannont subscribe to %s, server does not support shared subscriptions", sub.Topic)
			}
		}
	}

	c.debug.Printf("subscribing to %+v", s.Subscriptions)

	subCtx, cf := context.WithTimeout(ctx, c.PacketTimeout)
	defer cf()
	cpCtx := &CPContext{subCtx, make(chan packets.ControlPacket, 1)}

	sp := s.Packet()

	mid, err := c.MIDs.Request(cpCtx)
	if err != nil {
		return nil, err
	}
	defer c.MIDs.Free(mid)
	sp.PacketID = mid

	c.debug.Println("sending SUBSCRIBE")
	if _, err := sp.WriteTo(c.Conn); err != nil {
		return nil, err
	}
	c.debug.Println("waiting for SUBACK")
	var sap packets.ControlPacket

	select {
	case <-subCtx.Done():
		if ctxErr := subCtx.Err(); ctxErr != nil {
			c.debug.Println(fmt.Sprintf("terminated due to context: %v", ctxErr))
			return nil, ctxErr
		}
	case sap = <-cpCtx.Return:
	}

	if sap.Type != packets.SUBACK {
		return nil, fmt.Errorf("received %d instead of Suback", sap.Type)
	}
	c.debug.Println("received SUBACK")

	sa := SubackFromPacketSuback(sap.Content.(*packets.Suback))
	switch {
	case len(sa.Reasons) == 1:
		if sa.Reasons[0] >= 0x80 {
			var reason string
			c.debug.Println("received an error code in Suback:", sa.Reasons[0])
			if sa.Properties != nil {
				reason = sa.Properties.ReasonString
			}
			return sa, fmt.Errorf("failed to subscribe to topic: %s", reason)
		}
	default:
		for _, code := range sa.Reasons {
			if code >= 0x80 {
				c.debug.Println("received an error code in Suback:", code)
				return sa, fmt.Errorf("at least one requested subscription failed")
			}
		}
	}

	return sa, nil
}

// Unsubscribe is used to send an Unsubscribe request to the MQTT server.
// It is passed a pre-prepared Unsubscribe packet and blocks waiting for
// a response Unsuback, or for the timeout to fire. Any response Unsuback
// is returned from the function, along with any errors.
func (c *Client) Unsubscribe(ctx context.Context, u *Unsubscribe) (*Unsuback, error) {
	c.debug.Printf("unsubscribing from %+v", u.Topics)
	unsubCtx, cf := context.WithTimeout(ctx, c.PacketTimeout)
	defer cf()
	cpCtx := &CPContext{unsubCtx, make(chan packets.ControlPacket, 1)}

	up := u.Packet()

	mid, err := c.MIDs.Request(cpCtx)
	if err != nil {
		return nil, err
	}
	defer c.MIDs.Free(mid)
	up.PacketID = mid

	c.debug.Println("sending UNSUBSCRIBE")
	if _, err := up.WriteTo(c.Conn); err != nil {
		return nil, err
	}
	c.debug.Println("waiting for UNSUBACK")
	var uap packets.ControlPacket

	select {
	case <-unsubCtx.Done():
		if ctxErr := unsubCtx.Err(); ctxErr != nil {
			c.debug.Println(fmt.Sprintf("terminated due to context: %v", ctxErr))
			return nil, ctxErr
		}
	case uap = <-cpCtx.Return:
	}

	if uap.Type != packets.UNSUBACK {
		return nil, fmt.Errorf("received %d instead of Unsuback", uap.Type)
	}
	c.debug.Println("received SUBACK")

	ua := UnsubackFromPacketUnsuback(uap.Content.(*packets.Unsuback))
	switch {
	case len(ua.Reasons) == 1:
		if ua.Reasons[0] >= 0x80 {
			var reason string
			c.debug.Println("received an error code in Unsuback:", ua.Reasons[0])
			if ua.Properties != nil {
				reason = ua.Properties.ReasonString
			}
			return ua, fmt.Errorf("failed to unsubscribe from topic: %s", reason)
		}
	default:
		for _, code := range ua.Reasons {
			if code >= 0x80 {
				c.debug.Println("received an error code in Suback:", code)
				return ua, fmt.Errorf("at least one requested unsubscribe failed")
			}
		}
	}

	return ua, nil
}

// Publish is used to send a publication to the MQTT server.
// It is passed a pre-prepared Publish packet and blocks waiting for
// the appropriate response, or for the timeout to fire.
// Any response message is returned from the function, along with any errors.
func (c *Client) Publish(ctx context.Context, p *Publish) (*PublishResponse, error) {
	if p.QoS > c.serverProps.MaximumQoS {
		return nil, fmt.Errorf("cannot send Publish with QoS %d, server maximum QoS is %d", p.QoS, c.serverProps.MaximumQoS)
	}
	if p.Properties != nil && p.Properties.TopicAlias != nil {
		if c.serverProps.TopicAliasMaximum > 0 && *p.Properties.TopicAlias > c.serverProps.TopicAliasMaximum {
			return nil, fmt.Errorf("cannot send publish with TopicAlias %d, server topic alias maximum is %d", *p.Properties.TopicAlias, c.serverProps.TopicAliasMaximum)
		}
	}
	if !c.serverProps.RetainAvailable && p.Retain {
		return nil, fmt.Errorf("cannot send Publish with retain flag set, server does not support retained messages")
	}
	if (p.Properties == nil || p.Properties.TopicAlias == nil) && p.Topic == "" {
		return nil, fmt.Errorf("cannot send a publish with no TopicAlias and no Topic set")
	}

	if c.ClientConfig.PublishHook != nil {
		c.ClientConfig.PublishHook(p)
	}

	c.debug.Printf("sending message to %s", p.Topic)

	pb := p.Packet()

	switch p.QoS {
	case 0:
		c.debug.Println("sending QoS0 message")
		if _, err := pb.WriteTo(c.Conn); err != nil {
			return nil, err
		}
		return nil, nil
	case 1, 2:
		return c.publishQoS12(ctx, pb)
	}

	return nil, fmt.Errorf("QoS isn't 0, 1 or 2")
}

func (c *Client) publishQoS12(ctx context.Context, pb *packets.Publish) (*PublishResponse, error) {
	c.debug.Println("sending QoS12 message")
	pubCtx, cf := context.WithTimeout(ctx, c.PacketTimeout)
	defer cf()
	if err := c.serverInflight.Acquire(pubCtx, 1); err != nil {
		return nil, err
	}
	defer c.serverInflight.Release(1)
	cpCtx := &CPContext{pubCtx, make(chan packets.ControlPacket, 1)}

	mid, err := c.MIDs.Request(cpCtx)
	if err != nil {
		return nil, err
	}
	defer c.MIDs.Free(mid)
	pb.PacketID = mid

	if _, err := pb.WriteTo(c.Conn); err != nil {
		return nil, err
	}
	var resp packets.ControlPacket

	select {
	case <-pubCtx.Done():
		if ctxErr := pubCtx.Err(); ctxErr != nil {
			c.debug.Println(fmt.Sprintf("terminated due to context: %v", ctxErr))
			return nil, ctxErr
		}
	case resp = <-cpCtx.Return:
	}

	switch pb.QoS {
	case 1:
		if resp.Type != packets.PUBACK {
			return nil, fmt.Errorf("received %d instead of PUBACK", resp.Type)
		}

		pr := PublishResponseFromPuback(resp.Content.(*packets.Puback))
		if pr.ReasonCode >= 0x80 {
			c.debug.Println("received an error code in Puback:", pr.ReasonCode)
			return pr, fmt.Errorf("error publishing: %s", resp.Content.(*packets.Puback).Reason())
		}
		return pr, nil
	case 2:
		switch resp.Type {
		case packets.PUBCOMP:
			pr := PublishResponseFromPubcomp(resp.Content.(*packets.Pubcomp))
			return pr, nil
		case packets.PUBREC:
			c.debug.Printf("received PUBREC for %s (must have errored)", pb.PacketID)
			pr := PublishResponseFromPubrec(resp.Content.(*packets.Pubrec))
			return pr, nil
		default:
			return nil, fmt.Errorf("received %d instead of PUBCOMP", resp.Type)
		}
	}

	c.debug.Println("ended up with a non QoS1/2 message:", pb.QoS)
	return nil, fmt.Errorf("ended up with a non QoS1/2 message: %d", pb.QoS)
}

func (c *Client) expectConnack(packet chan<- *packets.Connack, errs chan<- error) {
	recv, err := packets.ReadPacket(c.Conn)
	if err != nil {
		errs <- err
		return
	}
	switch r := recv.Content.(type) {
	case *packets.Connack:
		c.debug.Println("received CONNACK")
		if r.ReasonCode == packets.ConnackSuccess && r.Properties != nil && r.Properties.AuthMethod != "" {
			// Successful connack and AuthMethod is defined, must have successfully authed during connect
			go c.AuthHandler.Authenticated()
		}
		packet <- r
	case *packets.Auth:
		c.debug.Println("received AUTH")
		if c.AuthHandler == nil {
			errs <- fmt.Errorf("enhanced authentication flow started but no AuthHandler configured")
			return
		}
		c.debug.Println("sending AUTH")
		_, err := c.AuthHandler.Authenticate(AuthFromPacketAuth(r)).Packet().WriteTo(c.Conn)
		if err != nil {
			errs <- fmt.Errorf("error sending authentication packet: %w", err)
			return
		}
		// go round again, either another AUTH or CONNACK
		go c.expectConnack(packet, errs)
	default:
		errs <- fmt.Errorf("received unexpected packet %v", recv.Type)
	}

}

// Disconnect is used to send a Disconnect packet to the MQTT server
// Whether or not the attempt to send the Disconnect packet fails
// (and if it does this function returns any error) the network connection
// is closed.
func (c *Client) Disconnect(d *Disconnect) error {
	c.debug.Println("disconnecting")
	_, err := d.Packet().WriteTo(c.Conn)

	c.close()
	c.workers.Wait()

	return err
}

// SetDebugLogger takes an instance of the paho Logger interface
// and sets it to be used by the debug log endpoint
func (c *Client) SetDebugLogger(l Logger) {
	c.debug = l
}

// SetErrorLogger takes an instance of the paho Logger interface
// and sets it to be used by the error log endpoint
func (c *Client) SetErrorLogger(l Logger) {
	c.errors = l
}
