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
	"testing"
)

func TestBoolToByte(t *testing.T) {
	if boolToByte(false) != 0x00 {
		t.Fail()
	}

	if boolToByte(true) != 0x01 {
		t.Fail()
	}
}

func TestGetRawProps(t *testing.T) {

	props := []byte{
		propKeyPayloadFormatIndicator, 1,
		propKeyMessageExpiryInterval, 1, 1, 1, 1,
		propKeyContentType, 0, 4, 'M', 'Q', 'T', 'T',
		propKeyRespTopic, 0, 4, 'M', 'Q', 'T', 'T',
		propKeyCorrelationData, 0, 4, 'M', 'Q', 'T', 'T',
		propKeySubID,
		propKeySessionExpiryInterval, 1, 1, 1, 1,
		propKeyAssignedClientID, 0, 4, 'M', 'Q', 'T', 'T',
		propKeyServerKeepalive, 1, 1,
		propKeyAuthMethod, 0, 4, 'M', 'Q', 'T', 'T',
		propKeyAuthData, 0, 4, 'M', 'Q', 'T', 'T',
		propKeyReqProblemInfo, 1,
		propKeyWillDelayInterval, 1, 1, 1, 1,
		propKeyReqRespInfo, 1,
		propKeyRespInfo, 1,
		propKeyServerRef, 0, 4, 'M', 'Q', 'T', 'T',
		propKeyReasonString, 0, 4, 'M', 'Q', 'T', 'T',
		propKeyMaxRecv, 1, 1,
		propKeyMaxTopicAlias, 1, 1,
		propKeyTopicAlias, 1, 1,
		propKeyMaxQos, 1,
		propKeyRetainAvail, 1,
		propKeyMaxPacketSize, 1, 1, 1, 1,
		propKeyWildcardSubAvail, 1,
		propKeySubIDAvail, 1,
		propKeySharedSubAvail, 1,

		// user props
		propKeyUserProps, 0, 2, 'M', 'Q', 0, 2, 'M', 'Q',
		propKeyUserProps, 0, 2, 'M', 'Q', 0, 2, 'T', 'T',
		propKeyUserProps, 0, 2, 'T', 'T', 0, 4, 'M', 'Q', 'T', 'T',
	}

	payload := []byte{0, 0, 0, 0, 0, 0, 0, 0}

	buf := &bytes.Buffer{}
	// prop length
	writeVarInt(len(props), buf)
	buf.Write(props)
	buf.Write(payload)

	_, next, err := getRawProps(buf.Bytes())
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(payload, next) != 0 {
		t.Error("payload decode error")
		t.Log(next)
	}

}
