// Tests for messages parsing/serializing

package main

import "testing"

func compareMessages(m1 *WebsocketProtocolMessage, m2 *WebsocketProtocolMessage) bool {
	if m1.MessageType != m2.MessageType {
		return false
	}

	if (len(m1.Parameters) == 0 && len(m2.Parameters) != 0) || (len(m1.Parameters) != 0 && len(m2.Parameters) == 0) {
		return false
	}

	if len(m1.Parameters) == 0 && len(m2.Parameters) == 0 {
		return true
	}

	for k, v := range m1.Parameters {
		if m2.Parameters[k] != v {
			return false
		}
	}

	for k, v := range m2.Parameters {
		if m1.Parameters[k] != v {
			return false
		}
	}

	return true
}

func testMessageIntegrity(t *testing.T, m *WebsocketProtocolMessage) {
	serialized := m.Serialize()

	parsed := ParseWebsocketProtocolMessage(serialized)

	if !compareMessages(m, parsed) {
		t.Error("Message integrity is not preserved. Message: " + m.Serialize() + " | Parsed: " + parsed.Serialize())
	}
}

func TestWebsocketProtocolMessage(t *testing.T) {
	// Test message integrity
	// meaning serialize -> deserialize does not change the message

	testMessageIntegrity(t, &WebsocketProtocolMessage{
		MessageType: "H",
	})

	testMessageIntegrity(t, &WebsocketProtocolMessage{
		MessageType: "PUSH",
		Parameters: map[string]string{
			"stream": "stream-id",
			"auth":   "auth-token-example",
		},
	})

	testMessageIntegrity(t, &WebsocketProtocolMessage{
		MessageType: "PULL",
		Parameters: map[string]string{
			"stream":                "stream-id",
			"auth":                  "auth-token-example",
			"only_source":           "true",
			"max_initial_fragments": "10",
		},
	})

	testMessageIntegrity(t, &WebsocketProtocolMessage{
		MessageType: "OK",
	})

	testMessageIntegrity(t, &WebsocketProtocolMessage{
		MessageType: "E",
		Parameters: map[string]string{
			"code":    "Error_Code",
			"message": "Example error message",
		},
	})

	testMessageIntegrity(t, &WebsocketProtocolMessage{
		MessageType: "F",
		Parameters: map[string]string{
			"duration": "2.5",
		},
	})

	testMessageIntegrity(t, &WebsocketProtocolMessage{
		MessageType: "CLOSE",
	})
}
