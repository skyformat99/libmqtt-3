package libmqtt

import (
	"bufio"
	"net"
	"time"
)

// clientConn is the wrapper of connection to server
// tend to actual packet send and receive
type clientConn struct {
	parent     *client       // client which created this connection
	name       string        // server addr info
	conn       net.Conn      // connection to server
	connW      *bufio.Writer // make buffered connection
	logicSendC chan Packet   // logic send channel
	netRecvC   chan Packet   // received packet from server
	keepaliveC chan int      // keepalive packet
}

// start mqtt logic
func (c *clientConn) logic() {
	defer c.parent.workers.Done()
	defer lg.i("CONN exit logic for server =", c.name)
	// start keepalive if required
	if c.parent.options.keepalive > 0 {
		c.parent.workers.Add(1)
		go c.keepalive()
	}

	for {
		select {
		case <-c.parent.exit:
			c.conn.Close()
			return
		case pkt, more := <-c.netRecvC:
			if !more {
				return
			}

			switch pkt.Type() {
			case CtrlSubAck:
				p := pkt.(*SubAckPacket)
				lg.d("NET received SubAck, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *SubscribePacket:
						originSub := originPkt.(*SubscribePacket)
						N := len(p.Codes)
						for i, v := range originSub.Topics {
							if i < N {
								v.Qos = p.Codes[i]
							}
						}
						lg.d("CLIENT subscribed topics =", originSub.Topics)
						c.parent.msgC <- newSubMsg(originSub.Topics, nil)
						c.parent.idGen.free(p.PacketID)

						if err := c.parent.persist.Delete(sendKey(p.PacketID)); err != nil {
							c.parent.msgC <- newPersistMsg(err)
						}
					}
				}
			case CtrlUnSubAck:
				p := pkt.(*UnSubAckPacket)
				lg.d("NET received UnSubAck, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *UnSubPacket:
						originUnSub := originPkt.(*UnSubPacket)
						lg.d("CLIENT unSubscribed topics", originUnSub.TopicNames)
						c.parent.msgC <- newUnSubMsg(originUnSub.TopicNames, nil)
						c.parent.idGen.free(p.PacketID)

						if err := c.parent.persist.Delete(sendKey(p.PacketID)); err != nil {
							c.parent.msgC <- newPersistMsg(err)
						}
					}
				}
			case CtrlPublish:
				p := pkt.(*PublishPacket)
				lg.d("NET received publish, topic =", p.TopicName, "id =", p.PacketID, "QoS =", p.Qos)
				// received server publish, send to client
				c.parent.recvC <- p

				// tend to QoS
				switch p.Qos {
				case Qos1:
					lg.d("NET send PubAck for Publish, id =", p.PacketID)
					c.send(&PubAckPacket{PacketID: p.PacketID})

					if err := c.parent.persist.Store(recvKey(p.PacketID), pkt); err != nil {
						c.parent.msgC <- newPersistMsg(err)
					}
				case Qos2:
					lg.d("NET send PubRecv for Publish, id =", p.PacketID)
					c.send(&PubRecvPacket{PacketID: p.PacketID})

					if err := c.parent.persist.Store(recvKey(p.PacketID), pkt); err != nil {
						c.parent.msgC <- newPersistMsg(err)
					}
				}
			case CtrlPubAck:
				p := pkt.(*PubAckPacket)
				lg.d("NET received PubAck, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *PublishPacket:
						originPub := originPkt.(*PublishPacket)
						if originPub.Qos == Qos1 {
							lg.d("CLIENT published qos1 packet, topic =", originPub.TopicName)
							c.parent.msgC <- newPubMsg(originPub.TopicName, nil)
							c.parent.idGen.free(p.PacketID)

							if err := c.parent.persist.Delete(sendKey(p.PacketID)); err != nil {
								c.parent.msgC <- newPersistMsg(err)
							}
						}
					}
				}
			case CtrlPubRecv:
				p := pkt.(*PubRecvPacket)
				lg.d("NET received PubRec, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *PublishPacket:
						originPub := originPkt.(*PublishPacket)
						if originPub.Qos == Qos2 {
							c.send(&PubRelPacket{PacketID: p.PacketID})
							lg.d("NET send PubRel, id =", p.PacketID)
						}
					}
				}
			case CtrlPubRel:
				p := pkt.(*PubRelPacket)
				lg.d("NET send PubRel, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *PublishPacket:
						originPub := originPkt.(*PublishPacket)
						if originPub.Qos == Qos2 {
							c.send(&PubCompPacket{PacketID: p.PacketID})
							lg.d("NET send PubComp, id =", p.PacketID)

							if err := c.parent.persist.Store(recvKey(p.PacketID), pkt); err != nil {
								c.parent.msgC <- newPersistMsg(err)
							}
						}
					}
				}
			case CtrlPubComp:
				p := pkt.(*PubCompPacket)
				lg.d("NET received PubComp, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					switch originPkt.(type) {
					case *PublishPacket:
						originPub := originPkt.(*PublishPacket)
						if originPub.Qos == Qos2 {
							c.send(&PubRelPacket{PacketID: p.PacketID})
							lg.d("NET send PubRel, id =", p.PacketID)
							lg.d("CLIENT published qos2 packet, topic =", originPub.TopicName)
							c.parent.msgC <- newPubMsg(originPub.TopicName, nil)
							c.parent.idGen.free(p.PacketID)

							if err := c.parent.persist.Delete(sendKey(p.PacketID)); err != nil {
								c.parent.msgC <- newPersistMsg(err)
							}
						}
					}
				}
			default:
				lg.d("NET received packet, type =", pkt.Type())
			}
		}
	}
}

// keepalive with server
func (c *clientConn) keepalive() {
	defer c.parent.workers.Done()

	lg.d("NET start keepalive")
	defer lg.d("NET stop keepalive for server =", c.name)

	t := time.NewTicker(c.parent.options.keepalive * 3 / 4)
	timeout := time.Duration(float64(c.parent.options.keepalive) * c.parent.options.keepaliveFactor)
	timeoutTimer := time.NewTimer(timeout)
	defer t.Stop()

	for {
		select {
		case <-c.parent.exit:
			return
		case <-t.C:
			c.send(PingReqPacket)

			select {
			case <-c.parent.exit:
				return
			case _, more := <-c.keepaliveC:
				if !more {
					return
				}

				timeoutTimer.Reset(timeout)
			case <-timeoutTimer.C:
				lg.i("NET keepalive timeout")
				return
			}
		}
	}
}

// handle mqtt logic control packet send
func (c *clientConn) handleSend() {
	defer c.parent.workers.Done()
	defer lg.i("CONN exit send handler for server =", c.name)

	for {
		select {
		case <-c.parent.exit:
			c.conn.Close()
			return
		case pkt, more := <-c.parent.sendC:
			if !more {
				return
			}

			if err := EncodeOnePacket(c.parent.options.protoVersion, pkt, c.connW); err != nil {
				lg.e("NET encode error", err)
				return
			}

			if err := c.connW.Flush(); err != nil {
				lg.e("NET flush error", err)
				return
			}

			switch pkt.Type() {
			case CtrlPublish:
				p := pkt.(*PublishPacket)
				if p.Qos == 0 {
					lg.d("CLIENT published qos0 packet, topic =", p.TopicName)
					c.parent.msgC <- newPubMsg(p.TopicName, nil)
				}
			case CtrlDisConn:
				// client exit with disconn
				close(c.parent.exit)
				return
			}
		case pkt, more := <-c.logicSendC:
			if !more {
				return
			}

			if err := EncodeOnePacket(c.parent.options.protoVersion, pkt, c.connW); err != nil {
				lg.e("NET encode error", err)
				return
			}

			if err := c.connW.Flush(); err != nil {
				lg.e("NET flush error", err)
				return
			}

			switch pkt.Type() {
			case CtrlPubRel:
				if err := c.parent.persist.Store(sendKey(pkt.(*PubRelPacket).PacketID), pkt); err != nil {
					c.parent.msgC <- newPersistMsg(err)
				}
			case CtrlPubAck:
				if err := c.parent.persist.Delete(sendKey(pkt.(*PubAckPacket).PacketID)); err != nil {
					c.parent.msgC <- newPersistMsg(err)
				}
			case CtrlPubComp:
				if err := c.parent.persist.Delete(sendKey(pkt.(*PubCompPacket).PacketID)); err != nil {
					c.parent.msgC <- newPersistMsg(err)
				}
			case CtrlDisConn:
				// disconnect to server
				c.conn.Close()
				return
			}
		}
	}
}

// handle all message receive
func (c *clientConn) handleRecv() {
	defer c.parent.workers.Done()
	defer lg.i("CONN exit recv handler for server =", c.name)

	for {
		select {
		case <-c.parent.exit:
			return
		default:
			pkt, err := DecodeOnePacket(c.parent.options.protoVersion, c.conn)
			if err != nil {
				lg.e("NET connection broken, server =", c.name, "err =", err)

				// notify net receive no packet will coming
				close(c.netRecvC)

				// notify keepalive worker to exit
				close(c.keepaliveC)

				// TODO send proper net error to net handler
				//if err != ErrDecodeBadPacket {
				//	c.parent.msgC <- newNetMsg(c.name, err)
				//}
				return
			}

			if pkt == PingRespPacket {
				lg.d("NET received keepalive message")
				c.keepaliveC <- 1
			} else {
				c.netRecvC <- pkt
			}
		}
	}
}

// send mqtt logic packet
func (c *clientConn) send(pkt Packet) {
	if !c.parent.isClosing() {
		c.logicSendC <- pkt
	}
}
