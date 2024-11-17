// Websocket message logic

package main

import (
	"net/url"
	"strings"
)

// Websocket protocol message
type WebsocketProtocolMessage struct {
	// Message type
	MessageType string

	// Message parameters
	Parameters map[string]string
}

// Gets the parameter value
func (msg *WebsocketProtocolMessage) GetParameter(param string) string {
	if msg.Parameters == nil {
		return ""
	}

	return msg.Parameters[param]
}

// Serializes message to string (to be sent)
func (msg *WebsocketProtocolMessage) Serialize() string {
	if msg.Parameters == nil || len(msg.Parameters) == 0 {
		return msg.MessageType
	}

	paramStr := ""

	for k, v := range msg.Parameters {
		if len(paramStr) > 0 {
			paramStr += "&"
		}

		paramStr += url.QueryEscape(k) + "=" + url.QueryEscape(v)
	}

	return msg.MessageType + ":" + paramStr
}

// Parses websocket protocol message from string
func ParseWebsocketProtocolMessage(str string) *WebsocketProtocolMessage {
	colonIndex := strings.IndexRune(str, ':')

	if colonIndex < 0 || colonIndex >= len(str)-1 {
		return &WebsocketProtocolMessage{
			MessageType: strings.ToUpper(str),
		}
	}

	msgType := strings.ToUpper(str[0:colonIndex])
	msgParams := str[colonIndex+1:]

	q, err := url.ParseQuery(msgParams)

	if err != nil {
		return &WebsocketProtocolMessage{
			MessageType: msgType,
		}
	}

	params := make(map[string]string)

	for k, v := range q {
		params[k] = strings.Join(v, "")
	}

	return &WebsocketProtocolMessage{
		MessageType: msgType,
		Parameters:  params,
	}
}
