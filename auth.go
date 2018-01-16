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

// AuthCode authentication result code
type AuthCode = byte

const (
	// AuthSuccess Success, sent by server
	AuthSuccess AuthCode = 0

	// AuthContinue Continue authentication, sent by client or server
	AuthContinue AuthCode = 24

	// ReAuth Re-authentication
	ReAuth AuthCode = 25
)

// AuthPacket Client <-> Server
// as part of an extended authentication exchange,
// such as challenge / response authentication.
//
// It is a Protocol Error for the Client or Server to send
// an AUTH packet if the ConnPacket did not contain the
// same Authentication Method
type AuthPacket struct {
	// Code the authentication result code
	Code AuthCode
	// Props authentication properties
	Props *AuthProps
}

// Type of AuthPacket is CtrlAuth
func (a *AuthPacket) Type() CtrlType {
	return CtrlAuth
}

// AuthProps properties of AuthPacket
type AuthProps struct {
	AuthMethod string         // 21
	AuthData   []byte         // 22
	Reason     string         // 31
	UserProps  UserProperties // 38
}

func (a *AuthProps) props() []byte {
	if a == nil {
		return nil
	}

	result := make([]byte, 0)
	if len(a.AuthMethod) != 0 {
		result = append(result, propKeyAuthMethod)
		result = append(result, encodeDataWithLen([]byte(a.AuthMethod))...)
	}

	if len(a.AuthData) != 0 {
		result = append(result, propKeyAuthData)
		result = append(result, encodeDataWithLen(a.AuthData)...)
	}

	if len(a.Reason) != 0 {
		result = append(result, propKeyReasonString)
		result = append(result, encodeDataWithLen([]byte(a.Reason))...)
	}

	if a.UserProps != nil {
		a.UserProps.encodeTo(result)
	}

	return result
}

func (a *AuthProps) setProps(props map[byte][]byte) {
	if a == nil {
		return
	}

	if v, ok := props[propKeyAuthMethod]; ok {
		a.AuthMethod, _, _ = decodeString(v)
	}

	if v, ok := props[propKeyAuthData]; ok {
		a.AuthData, _, _ = decodeData(v)
	}

	if v, ok := props[propKeyReasonString]; ok {
		a.Reason, _, _ = decodeString(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		a.UserProps = decodeUserProps(v)
	}
}
