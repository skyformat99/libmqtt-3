/*
 * Copyright GoIIoT (https://github.com/goiiot)
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
	"errors"
	"io"
)

var (
	// ErrDecodeBadPacket is the error happened when trying to decode a none MQTT packet
	ErrDecodeBadPacket = errors.New("decoded none MQTT packet ")
)

// DecodeOnePacket will decode one mqtt packet
func DecodeOnePacket(version ProtocolVersion, reader io.Reader) (Packet, error) {
	switch version {
	case V311:
		return decodeV311Packet(reader)
	case V5:
		return decodeV5Packet(reader)
	default:
		return nil, errUnsupportedVersion
	}
}

func decodeString(data []byte) (d string, next []byte, err error) {
	var b []byte
	b, next, err = decodeData(data)
	if err == nil {
		d = string(b)
	}

	return
}

func decodeData(data []byte) (d []byte, next []byte, err error) {
	if len(data) < 2 {
		return nil, nil, ErrDecodeBadPacket
	}
	length := int(data[0])<<8 + int(data[1])
	if length+2 > len(data) {
		// out of bounds
		return nil, nil, ErrDecodeBadPacket
	}
	return data[2 : length+2], data[length+2:], nil
}

func decodeRemainLength(reader io.Reader) (result int, err error) {
	buf := make([]byte, 1)
	_, err = io.ReadFull(reader, buf[:])
	result = int(buf[0] & 127)
	m := 1
	for (buf[0] & 0x80) != 0 {
		_, err = io.ReadFull(reader, buf[:])
		if err != nil {
			return
		}

		result += int(buf[0]&127) * m
		m <<= 8

		if m > 128*128*128 {
			return
		}
	}

	return
}

func decodeV311Packet(reader io.Reader) (Packet, error) {
	headerBytes := make([]byte, 1)
	var err error
	if _, err = io.ReadFull(reader, headerBytes[:]); err != nil {
		return nil, err
	}

	var bytesToRead int
	if bytesToRead, err = decodeRemainLength(reader); err != nil {
		return nil, err
	} else if bytesToRead == 0 {
		switch headerBytes[0] >> 4 {
		case CtrlPingReq:
			return PingReqPacket, nil
		case CtrlPingResp:
			return PingRespPacket, nil
		case CtrlDisConn:
			return DisConnPacket, nil
		default:
			return nil, ErrDecodeBadPacket
		}
	} else if bytesToRead < 2 {
		return nil, ErrDecodeBadPacket
	}

	body := make([]byte, bytesToRead)
	if _, err = io.ReadFull(reader, body[:]); err != nil {
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

		if len(next) < 4 {
			return nil, ErrDecodeBadPacket
		}
		hasUsername := next[1]&0x80 == 0x80
		hasPassword := next[1]&0x40 == 0x40
		tmpPkt := &ConnPacket{
			protoName:    protocol,
			protoLevel:   next[0],
			CleanSession: next[1]&0x02 == 0x02,
			IsWill:       next[1]&0x04 == 0x04,
			WillQos:      next[1] & 0x18 >> 3,
			WillRetain:   next[1]&0x20 == 0x20,
			Keepalive:    uint16(next[2])<<8 + uint16(next[3]),
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
			pub.PacketID = uint16(next[0])<<8 + uint16(next[1])
			next = next[2:]
		}

		pub.Payload = next
		return pub, nil
	case CtrlPubAck:
		return &PubAckPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	case CtrlPubRecv:
		return &PubRecvPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	case CtrlPubRel:
		return &PubRelPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	case CtrlPubComp:
		return &PubCompPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	case CtrlSubscribe:
		pktTmp := &SubscribePacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}

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
		pktTmp := &SubAckPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}

		next = body[2:]
		codes := make([]SubAckCode, 0)
		for i := 0; i < len(next); i++ {
			codes = append(codes, next[i])
		}
		return pktTmp, nil
	case CtrlUnSub:
		pktTmp := &UnSubPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}
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
		return &UnSubAckPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	}

	return nil, ErrDecodeBadPacket
}

func decodeV5Packet(reader io.Reader) (Packet, error) {
	headerBytes := make([]byte, 1)
	var err error
	if _, err = io.ReadFull(reader, headerBytes[:]); err != nil {
		return nil, err
	}

	var bytesToRead int
	if bytesToRead, err = decodeRemainLength(reader); err != nil {
		return nil, err
	} else if bytesToRead == 0 {
		switch headerBytes[0] >> 4 {
		case CtrlPingReq:
			return PingReqPacket, nil
		case CtrlPingResp:
			return PingRespPacket, nil
		case CtrlDisConn:
			return DisConnPacket, nil
		default:
			return nil, ErrDecodeBadPacket
		}
	} else if bytesToRead < 2 {
		return nil, ErrDecodeBadPacket
	}

	body := make([]byte, bytesToRead)
	if _, err = io.ReadFull(reader, body[:]); err != nil {
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

		if len(next) < 4 {
			return nil, ErrDecodeBadPacket
		}
		hasUsername := next[1]&0x80 == 0x80
		hasPassword := next[1]&0x40 == 0x40
		tmpPkt := &ConnPacket{
			protoName:    protocol,
			protoLevel:   next[0],
			CleanSession: next[1]&0x02 == 0x02,
			IsWill:       next[1]&0x04 == 0x04,
			WillQos:      next[1] & 0x18 >> 3,
			WillRetain:   next[1]&0x20 == 0x20,
			Keepalive:    uint16(next[2])<<8 + uint16(next[3]),
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
		return &ConnAckPacket{
			Present: body[0]&0x01 == 0x01,
			Code:    body[1],
		}, nil
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
			pub.PacketID = uint16(next[0])<<8 + uint16(next[1])
			next = next[2:]
		}

		pub.Payload = next
		return pub, nil
	case CtrlPubAck:
		return &PubAckPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	case CtrlPubRecv:
		return &PubRecvPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	case CtrlPubRel:
		return &PubRelPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	case CtrlPubComp:
		return &PubCompPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	case CtrlSubscribe:
		pktTmp := &SubscribePacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}

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
		pktTmp := &SubAckPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}

		next = body[2:]
		codes := make([]SubAckCode, 0)
		for i := 0; i < len(next); i++ {
			codes = append(codes, next[i])
		}
		return pktTmp, nil
	case CtrlUnSub:
		pktTmp := &UnSubPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}
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
		return &UnSubAckPacket{PacketID: uint16(body[0])<<8 + uint16(body[1])}, nil
	case CtrlAuth:
		//return
	}

	return nil, ErrDecodeBadPacket
}
