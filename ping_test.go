package libmqtt

import (
	"testing"
)

func TestPingReqPacket_Bytes(t *testing.T) {
	testV311Bytes(testPingReqMsg, testPingReqMsgBytes, t)
}

func TestPingRespPacket_Bytes(t *testing.T) {
	testV311Bytes(testPingRespMsg, testPingRespMsgBytes, t)
}
