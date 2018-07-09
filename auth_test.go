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
	"testing"
)

const (
	testAuthMethod = "test auth method"
	testAuthData   = "test auth data"
	testAuthReason = "test auth reason"
)

var (
	testAuthMsg      = &AuthPacket{}
	testAuthMsgBytes = []byte{}

	testAuthProps = &AuthProps{
		AuthMethod: testAuthMethod,
		AuthData:   []byte(testAuthData),
		Reason:     testAuthReason,
		UserProps:  UserProps{"1": []string{"1_1", "1_2"}, "2": []string{"2_1", "2_2"}},
	}
	testProps = map[byte][]byte{
		propKeyAuthMethod:   []byte(testAuthMethod),
		propKeyAuthData:     []byte(testAuthData),
		propKeyReasonString: []byte(testAuthReason),
		propKeyUserProps:    []byte{},
	}
	testAuthPropsBytes = []byte{}
)

func initTestData_Auth() {

}

func TestAuthPacket_Bytes(t *testing.T) {
	// testV5Bytes(testAuthMsg, testAuthMsgBytes, t)
}

func TestAuthProps_Props(t *testing.T) {
	// propsBytes := testAuthProps.props()
	// if bytes.Compare(propsBytes, testAuthPropsBytes) != 0 {
	// t.Errorf("auth props bytes not math:\ntarget: %v\ngenerated: %v", testAuthPropsBytes, propsBytes)
	// }
}

func TestAuthProps_SetProps(t *testing.T) {
	// emptyProps := &AuthProps{}
	// emptyProps.setProps(testProps)

	// if emptyProps.AuthMethod != testAuthProps.AuthMethod {
	// 	t.Error("auth method set failed")
	// }

	// if bytes.Compare(emptyProps.AuthData, testAuthProps.AuthData) != 0 {
	// 	t.Error("auth data set failed")
	// }

	// if emptyProps.Reason != testAuthProps.Reason {
	// 	t.Error("auth reason set failed")
	// }

	// for k, v := range emptyProps.UserProps {
	// 	if tv, ok := testAuthProps.UserProps[k]; ok {
	// 		if len(v) == len(tv) {
	// 			for i := range v {
	// 				if v[i] != tv[i] {
	// 					t.Error("auth user props set failed")
	// 				}
	// 			}
	// 			continue
	// 		}
	// 	}
	// }
}
