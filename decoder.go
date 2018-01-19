/*
 * Copyright Go-IIoT (https://github.com/goiiot)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package libmqtt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

var (
	// ErrDecodeBadPacket is the error happened when trying to decode a none MQTT packet
	ErrDecodeBadPacket = errors.New("decoded none MQTT packet ")

	// ErrDecodeNoneV311Packet is the error happened when
	// trying to decode mqtt 3.1.1 packet but got other mqtt packet version
	ErrDecodeNoneV311Packet = errors.New("decoded none MQTT v3.1.1 packet ")

	// ErrDecodeNoneV5Packet is the error happened when
	// trying to decode mqtt 5 packet but got other mqtt packet version
	ErrDecodeNoneV5Packet = errors.New("decoded none MQTT v5 packet ")
)

// DecodeOnePacket will decode one mqtt packet
func DecodeOnePacket(version ProtoVersion, reader io.Reader) (Packet, error) {
	switch version {
	case V311:
		return decodeV311Packet(reader)
	case V5:
		return decodeV5Packet(reader)
	default:
		return nil, errUnsupportedVersion
	}
}

func decodeString(data []byte) (string, []byte, error) {
	b, next, err := decodeData(data)
	if err == nil {
		return string(b), next, err
	}

	return "", next, err
}

func decodeData(data []byte) ([]byte, []byte, error) {
	if len(data) < 2 {
		return nil, nil, ErrDecodeBadPacket
	}

	length := int(decodeUint16(data))
	if length+2 > len(data) {
		// out of bounds
		return nil, nil, ErrDecodeBadPacket
	}
	return data[2 : length+2], data[length+2:], nil
}

func decodeRemainLength(r io.Reader) (int, int) {
	var length uint32
	var multiplier uint32
	b := make([]byte, 1)
	for multiplier < 27 {
		io.ReadFull(r, b)

		digit := b[0]
		length |= uint32(digit&127) << multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7
	}
	return int(length), int(multiplier/7 + 1)
}

// decode mqtt v3.1.1 packets
func decodeV311Packet(r io.Reader) (Packet, error) {
	headerBytes := make([]byte, 1)
	var err error
	if _, err = io.ReadFull(r, headerBytes[:]); err != nil {
		return nil, err
	}

	bytesToRead, _ := decodeRemainLength(r)
	if bytesToRead == 0 {
		switch headerBytes[0] >> 4 {
		case CtrlPingReq:
			return PingReqPacket, nil
		case CtrlPingResp:
			return PingRespPacket, nil
		case CtrlDisConn:
			return &DisConnPacket{}, nil
		default:
			return nil, ErrDecodeBadPacket
		}
	} else if bytesToRead < 2 {
		return nil, ErrDecodeBadPacket
	}

	body := make([]byte, bytesToRead)
	if _, err = io.ReadFull(r, body[:]); err != nil {
		return nil, err
	}

	header := headerBytes[0]
	var next []byte
	switch header >> 4 {
	case CtrlConn:
		var protocol string
		if protocol, next, err = decodeString(body); err != nil {
			return nil, err
		}

		if next[0] != V311 {
			return nil, ErrDecodeNoneV311Packet
		}

		if len(next) < 4 {
			return nil, ErrDecodeBadPacket
		}
		hasUsername := next[1]&0x80 == 0x80
		hasPassword := next[1]&0x40 == 0x40
		tmpPkt := &ConnPacket{
			ProtoName:    protocol,
			Version:      next[0],
			CleanSession: next[1]&0x02 == 0x02,
			IsWill:       next[1]&0x04 == 0x04,
			WillQos:      next[1] & 0x18 >> 3,
			WillRetain:   next[1]&0x20 == 0x20,
			Keepalive:    decodeUint16(next[2:4]),
		}
		if tmpPkt.ClientID, next, err = decodeString(next[4:]); err != nil {
			return nil, err
		}

		if tmpPkt.IsWill {
			tmpPkt.WillTopic, next, err = decodeString(next)
			tmpPkt.WillMessage, next, err = decodeData(next)
		}

		if hasUsername {
			tmpPkt.Username, next, err = decodeString(next)
		}

		if hasPassword {
			tmpPkt.Password, _, err = decodeString(next)
		}

		if err != nil {
			return nil, err
		}

		return tmpPkt, nil
	case CtrlConnAck:
		return &ConnAckPacket{Present: body[0]&0x01 == 0x01, Code: body[1]}, nil
	case CtrlPublish:
		var topicName string
		if topicName, next, err = decodeString(body); err != nil {
			return nil, err
		}

		if len(next) < 2 {
			return nil, ErrDecodeBadPacket
		}

		pub := &PublishPacket{
			IsDup:     header&0x08 == 0x08,
			Qos:       header & 0x06 >> 1,
			IsRetain:  header&0x01 == 1,
			TopicName: topicName,
		}

		if pub.Qos > Qos0 {
			pub.PacketID = decodeUint16(next)
			next = next[2:]
		}

		pub.Payload = next
		return pub, nil
	case CtrlPubAck:
		return &PubAckPacket{PacketID: decodeUint16(body)}, nil
	case CtrlPubRecv:
		return &PubRecvPacket{PacketID: decodeUint16(body)}, nil
	case CtrlPubRel:
		return &PubRelPacket{PacketID: decodeUint16(body)}, nil
	case CtrlPubComp:
		return &PubCompPacket{PacketID: decodeUint16(body)}, nil
	case CtrlSubscribe:
		pktTmp := &SubscribePacket{PacketID: decodeUint16(body)}

		next = body[2:]
		topics := make([]*Topic, 0)
		for len(next) > 0 {
			var name string
			if name, next, err = decodeString(next); err != nil {
				return nil, err
			}

			if len(next) < 1 {
				return nil, ErrDecodeBadPacket
			}

			topics = append(topics, &Topic{Name: name, Qos: next[0]})
			next = next[1:]
		}
		pktTmp.Topics = topics
		return pktTmp, nil
	case CtrlSubAck:
		pktTmp := &SubAckPacket{PacketID: decodeUint16(body)}

		next = body[2:]
		codes := make([]SubAckCode, 0)
		for i := 0; i < len(next); i++ {
			codes = append(codes, next[i])
		}
		return pktTmp, nil
	case CtrlUnSub:
		pktTmp := &UnSubPacket{PacketID: decodeUint16(body)}
		next = body[2:]
		topics := make([]string, 0)
		for len(next) > 0 {
			var name string
			name, next, err = decodeString(next)
			if err != nil {
				return nil, err
			}
			topics = append(topics, name)
		}
		pktTmp.TopicNames = topics
		return pktTmp, nil
	case CtrlUnSubAck:
		return &UnSubAckPacket{PacketID: decodeUint16(body)}, nil
	}

	return nil, ErrDecodeBadPacket
}

// decode mqtt v5 packets
func decodeV5Packet(r io.Reader) (Packet, error) {
	headerBytes := make([]byte, 1)
	var err error
	if _, err = io.ReadFull(r, headerBytes[:]); err != nil {
		return nil, err
	}

	bytesToRead, _ := decodeRemainLength(r)
	if bytesToRead == 0 {
		switch headerBytes[0] >> 4 {
		case CtrlPingReq:
			return PingReqPacket, nil
		case CtrlPingResp:
			return PingRespPacket, nil
		default:
			return nil, ErrDecodeBadPacket
		}
	} else if bytesToRead < 2 {
		return nil, ErrDecodeBadPacket
	}

	body := make([]byte, bytesToRead)
	if _, err = io.ReadFull(r, body[:]); err != nil {
		return nil, err
	}

	header := headerBytes[0]
	var next []byte
	switch header >> 4 {
	case CtrlConn:
		var protocol string
		if protocol, next, err = decodeString(body); err != nil {
			return nil, err
		}

		if next[0] != V5 {
			return nil, ErrDecodeNoneV5Packet
		}

		if len(next) < 5 {
			return nil, ErrDecodeBadPacket
		}

		hasUsername := next[1]&0x80 == 0x80
		hasPassword := next[1]&0x40 == 0x40
		tmpPkt := &ConnPacket{
			ProtoName:    protocol,
			Version:      next[0],
			CleanSession: next[1]&0x02 == 0x02,
			IsWill:       next[1]&0x04 == 0x04,
			WillQos:      next[1] & 0x18 >> 3,
			WillRetain:   next[1]&0x20 == 0x20,
			Keepalive:    decodeUint16(next[2:4]),
		}
		// read properties
		var props map[byte][]byte
		props, next = decodeRawProps(next[4:])
		tmpPkt.Props = &ConnProps{}
		tmpPkt.Props.setProps(props)

		if tmpPkt.ClientID, next, err = decodeString(next); err != nil {
			return nil, err
		}

		if tmpPkt.IsWill {
			tmpPkt.WillTopic, next, err = decodeString(next)
			tmpPkt.WillMessage, next, err = decodeData(next)
		}

		if hasUsername {
			tmpPkt.Username, next, err = decodeString(next)
		}

		if hasPassword {
			tmpPkt.Password, _, err = decodeString(next)
		}

		if err != nil {
			return nil, err
		}

		return tmpPkt, nil
	case CtrlConnAck:
		pkt := &ConnAckPacket{
			Present: body[0]&0x01 == 0x01,
			Code:    body[1],
		}
		var props map[byte][]byte
		props, next = decodeRawProps(body[2:])
		pkt.Props = &ConnAckProps{}
		pkt.Props.setProps(props)
		return pkt, nil
	case CtrlPublish:
		var topicName string
		if topicName, next, err = decodeString(body); err != nil {
			return nil, err
		}

		if len(next) < 2 {
			return nil, ErrDecodeBadPacket
		}

		pub := &PublishPacket{
			IsDup:     header&0x08 == 0x08,
			Qos:       header & 0x06 >> 1,
			IsRetain:  header&0x01 == 1,
			TopicName: topicName,
		}

		if pub.Qos > Qos0 {
			pub.PacketID = decodeUint16(next)
			next = next[2:]
		}

		pub.Payload = next
		return pub, nil
	case CtrlPubAck:
		return &PubAckPacket{PacketID: decodeUint16(body)}, nil
	case CtrlPubRecv:
		return &PubRecvPacket{PacketID: decodeUint16(body)}, nil
	case CtrlPubRel:
		return &PubRelPacket{PacketID: decodeUint16(body)}, nil
	case CtrlPubComp:
		return &PubCompPacket{PacketID: decodeUint16(body)}, nil
	case CtrlSubscribe:
		pktTmp := &SubscribePacket{PacketID: decodeUint16(body)}

		next = body[2:]
		topics := make([]*Topic, 0)
		for len(next) > 0 {
			var name string
			if name, next, err = decodeString(next); err != nil {
				return nil, err
			}

			if len(next) < 1 {
				return nil, ErrDecodeBadPacket
			}

			topics = append(topics, &Topic{Name: name, Qos: next[0]})
			next = next[1:]
		}
		pktTmp.Topics = topics
		return pktTmp, nil
	case CtrlSubAck:
		pktTmp := &SubAckPacket{PacketID: decodeUint16(body)}

		next = body[2:]
		codes := make([]SubAckCode, 0)
		for i := 0; i < len(next); i++ {
			codes = append(codes, next[i])
		}
		return pktTmp, nil
	case CtrlUnSub:
		pktTmp := &UnSubPacket{PacketID: decodeUint16(body)}
		next = body[2:]
		topics := make([]string, 0)
		for len(next) > 0 {
			var name string
			name, next, err = decodeString(next)
			if err != nil {
				return nil, err
			}
			topics = append(topics, name)
		}
		pktTmp.TopicNames = topics
		return pktTmp, nil
	case CtrlUnSubAck:
		return &UnSubAckPacket{PacketID: decodeUint16(body)}, nil
	case CtrlAuth:
		return &AuthPacket{}, nil
	case CtrlDisConn:
		return &DisConnPacket{}, nil
	}

	return nil, ErrDecodeBadPacket
}

func decodeRawProps(data []byte) (map[byte][]byte, []byte) {
	propsLen, byteLen := decodeRemainLength(bytes.NewReader(data))
	propsBytes := data[1 : propsLen+byteLen]
	next := data[1+propsLen:]

	props := make(map[byte][]byte)
	for i := 0; i < propsLen; {
		var p []byte
		switch propsBytes[0] {
		case propKeyPayloadFormatIndicator:
			p = propsBytes[1:2]
		case propKeyMessageExpiryInterval:
			p = propsBytes[1:5]
		case propKeyContentType:
			p = propsBytes[1 : 3+decodeUint16(propsBytes[1:3])]
		case propKeyRespTopic:
			p = propsBytes[1 : 3+decodeUint16(propsBytes[1:3])]
		case propKeyCorrelationData:
			p = propsBytes[1 : 3+decodeUint16(propsBytes[1:3])]
		case propKeySubID:
			_, byteLen := decodeRemainLength(bytes.NewReader(propsBytes[1:]))
			p = propsBytes[1 : 1+byteLen]
		case propKeySessionExpiryInterval:
			p = propsBytes[1:5]
		case propKeyAssignedClientID:
			p = propsBytes[1 : 3+decodeUint16(propsBytes[1:3])]
		case propKeyServerKeepalive:
			p = propsBytes[1:3]
		case propKeyAuthMethod:
			p = propsBytes[1 : 3+decodeUint16(propsBytes[1:3])]
		case propKeyAuthData:
			p = propsBytes[1 : 3+decodeUint16(propsBytes[1:3])]
		case propKeyReqProblemInfo:
			p = propsBytes[1:2]
		case propKeyWillDelayInterval:
			p = propsBytes[1:5]
		case propKeyReqRespInfo:
			p = propsBytes[1:2]
		case propKeyRespInfo:
			p = propsBytes[1:2]
		case propKeyServerRef:
			p = propsBytes[1 : 3+decodeUint16(propsBytes[1:3])]
		case propKeyReasonString:
			p = propsBytes[1 : 3+decodeUint16(propsBytes[1:3])]
		case propKeyMaxRecv:
			p = propsBytes[1:3]
		case propKeyMaxTopicAlias:
			p = propsBytes[1:3]
		case propKeyTopicAlias:
			p = propsBytes[1:3]
		case propKeyMaxQos:
			p = propsBytes[1:2]
		case propKeyRetainAvail:
			p = propsBytes[1:2]
		case propKeyUserProps:
			keyEnd := 3 + decodeUint16(propsBytes[2:4])
			valEnd := keyEnd + decodeUint16(propsBytes[keyEnd:keyEnd+2])
			p = append(propsBytes[1:keyEnd], propsBytes[keyEnd:valEnd]...)
		case propKeyMaxPacketSize:
			p = propsBytes[1:5]
		case propKeyWildcardSubAvail:
			p = propsBytes[1:2]
		case propKeySubIDAvail:
			p = propsBytes[1:2]
		case propKeySharedSubAvail:
			p = propsBytes[1:2]
		}
		props[propsBytes[0]] = p
		propsBytes = propsBytes[1+len(p):]
		i += 1 + len(p)
	}

	return props, next
}

func decodeUserProps(data []byte) UserProperties {
	props := make(UserProperties)
	for str, next, _ := decodeString(data); next != nil; {
		var val string
		val, next, _ = decodeString(next)

		if _, ok := props[str]; ok {
			props[str] = append(props[str], val)
		} else {
			props[str] = []string{val}
		}
	}
	return props
}

func decodeUint16(d []byte) uint16 {
	return binary.BigEndian.Uint16(d)
}

func decodeUint32(d []byte) uint32 {
	return binary.BigEndian.Uint32(d)
}
