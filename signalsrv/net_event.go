package signalsrv

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"unicode/utf16"

	"github.com/pkg/errors"
)

const (
	NetEventTypeInvalid                   = 0
	NetEventTypeUnreliableMessageReceived = 1
	NetEventTypeReliableMessageReceived   = 2
	NetEventTypeServerInitialized         = 3
	NetEventTypeServerInitFailed          = 4
	NetEventTypeServerClosed              = 5
	NetEventTypeNewConnection             = 6
	NetEventTypeConnectionFailed          = 7
	NetEventTypeDisconnected              = 8
	NetEventTypeFatalError                = 100
	NetEventTypeWarning                   = 101
	NetEventTypeLog                       = 102
)

var NetEventTypeSTI = map[string]int{
	"Invalid":                   0,
	"UnreliableMessageReceived": 1,
	"ReliableMessageReceived":   2,
	"ServerInitialized":         3,
	"ServerInitFailed":          4,
	"ServerClosed":              5,
	"NewConnection":             6,
	"ConnectionFailed":          7,
	"Disconnected":              8,
	"FatalError":                100,
	"Warning":                   101,
	"Log":                       102,
}

var NetEventTypeITS = map[int]string{
	0:   "Invalid",
	1:   "UnreliableMessageReceived",
	2:   "ReliableMessageReceived",
	3:   "ServerInitialized",
	4:   "ServerInitFailed",
	5:   "ServerClosed",
	6:   "NewConnection",
	7:   "ConnectionFailed",
	8:   "Disconnected",
	100: "FatalError",
	101: "Warning",
	102: "Log",
}

type NetEventDataType int

const (
	NetEventDataTypeNull NetEventDataType = iota
	NetEventDataTypeByteArray
	NetEventDataTypeUTF16String
)

func (t NetEventDataType) String() string {
	switch t {
	case NetEventDataTypeNull:
		return "Null"
	case NetEventDataTypeByteArray:
		return "ByteArray"
	case NetEventDataTypeUTF16String:
		return "UTF16String"
	default:
		return "unknown"
	}
}

var NetEventDataTypeMap = map[string]int32{
	"Null":        0,
	"ByteArray":   1,
	"UTF16String": 2,
}

type NetEventData struct {
	Type       NetEventDataType
	StringData *string
	ObjectData []uint8
}

type ConnectionId struct {
	ID int16 `json:"id"`
}

func NewConnectionId(nid int16) *ConnectionId {
	return &ConnectionId{
		ID: nid,
	}
}

var INVALIDConnectionId = NewConnectionId(-1)

type NetworkEvent struct {
	Type         int           `json:"type"`
	ConnectionId *ConnectionId `json:"connectionId"`
	Data         *NetEventData `json:"data"`
}

func NewNetworkEvent(t int, conId *ConnectionId, data *NetEventData) *NetworkEvent {
	return &NetworkEvent{
		Type:         t,
		ConnectionId: conId,
		Data:         data,
	}
}

func (ne *NetworkEvent) GetRawData() *NetEventData {
	return ne.Data
}

func (ne *NetworkEvent) GetMessageData() *NetEventData {
	if ne.Data.Type != NetEventDataTypeUTF16String {
		return ne.Data
	}
	return nil
}

func (ne *NetworkEvent) GetInfo() *NetEventData {
	if ne.Data.Type == NetEventDataTypeUTF16String {
		return ne.Data
	}
	return nil
}

func (ne *NetworkEvent) String() (output string) {
	var data string
	if ne.Data.Type == NetEventDataTypeUTF16String && ne.Data.StringData != nil {
		data = *ne.Data.StringData
	} else if ne.Data.Type == NetEventDataTypeByteArray {
		d, err := toUint16Array(ne.Data.ObjectData)
		if err != nil {
			log.Println("parse data error: ", err)
		} else {
			data = string(utf16.Decode(d))
		}
	}
	output = fmt.Sprintf("NetworkEvent[NetEventType: (%s), id: (%d), Data: (%s)]",
		NetEventTypeITS[ne.Type], ne.ConnectionId.ID, data)

	return
}

type baseNetworkEvent struct {
	Type         int           `json:"type"`
	ConnectionId *ConnectionId `json:"connectionId"`
	Data         interface{}   `json:"data"`
}

func ParseFromString(str string) *NetworkEvent {
	evt := baseNetworkEvent{}
	err := json.Unmarshal([]byte(str), &evt)
	if err != nil {
		log.Printf("ParseFromString error. str: %s, err: %s", str, err.Error())
		return nil
	}
	if evt.ConnectionId == nil {
		log.Printf("ParseFromString error. str: %s, err: %s", str, "no connectionId")
		return nil
	}

	data := new(NetEventData)
	if evt.Data == nil || reflect.TypeOf(evt.Data).String() == "<nil>" {
		data.Type = NetEventDataTypeNull
	} else if reflect.TypeOf(evt.Data).String() == "string" {
		data.Type = NetEventDataTypeUTF16String
		s := evt.Data.(string)
		data.StringData = &s
	} else if reflect.TypeOf(evt.Data).String() == "[]interface {}" {
		data.Type = NetEventDataTypeByteArray
		for _, v := range evt.Data.([]interface{}) {
			data.ObjectData = append(data.ObjectData, uint8(v.(float64)))
		}
	} else {
		log.Println("data can't be parsed")
		return nil
	}

	return NewNetworkEvent(evt.Type, NewConnectionId(evt.ConnectionId.ID), data)
}

// 首先数据是小端字节序
// example: arr := []byte{3, 2, 255, 255, 3, 0, 0, 0, 49, 0, 50, 0, 51, 0}
// arr[0]为事件类型(event_type, uint8), arr[1]为数据类型(data_type, uint8)
// arr[2:4]为connection_id( int16 ), arr[4:8]为数据长度(data_length, uint32),
// arr[8:data_length]为数据(data, uint16)
// arr 转换为 event_type = 3, data_type = 2, connection_id = -1, data_length = 3, data = "123"
func FromByteArray(arr []byte) (*NetworkEvent, error) {
	typ := int(arr[0])
	dataType := arr[1]
	var id int16
	err := binary.Read(bytes.NewReader(arr[2:4]), binary.LittleEndian, &id)
	if err != nil {
		log.Println("parse id error: ", err)
		return nil, err
	}

	data := new(NetEventData)
	switch NetEventDataType(dataType) {
	case NetEventDataTypeByteArray:
		length := binary.LittleEndian.Uint32(arr[4:8])
		data.Type = NetEventDataTypeByteArray
		data.ObjectData = arr[8 : 8+length]
	case NetEventDataTypeUTF16String:
		length := binary.LittleEndian.Uint32(arr[4:8])
		d, err := toUint16Array(arr[8 : 8+length*2])
		if err != nil {
			log.Println("parse data error: ", err)
			return nil, err
		}
		data.Type = NetEventDataTypeUTF16String
		str := string(utf16.Decode(d))
		data.StringData = &str
	case NetEventDataTypeNull:
		data.Type = NetEventDataTypeNull
	default:
		log.Println("Message has an invalid data type flag: ", dataType)
		return nil, errors.New(fmt.Sprintf("Message has an invalid data type flag: %d", dataType))
	}

	return NewNetworkEvent(typ, NewConnectionId(id), data), nil
}

func toUint16Array(buf []byte) ([]uint16, error) {
	if len(buf)%2 != 0 {
		return nil, errors.New("trailing bytes")
	}
	vals := make([]uint16, len(buf)/2)
	for i := 0; i < len(vals); i++ {
		vals[i] = binary.LittleEndian.Uint16(buf[i*2:])
	}
	return vals, nil
}

func (ne *NetworkEvent) ToByteArray() []byte {
	var dataType NetEventDataType
	length := 4
	switch ne.Data.Type {
	case NetEventDataTypeByteArray:
		dataType = NetEventDataTypeByteArray
		length += len(ne.Data.ObjectData) + 4
	case NetEventDataTypeUTF16String:
		dataType = NetEventDataTypeUTF16String
		length += len([]rune(*ne.Data.StringData))*2 + 4
	default:
		dataType = NetEventDataTypeNull
	}

	result := make([]byte, length)
	result[0] = byte(ne.Type)
	result[1] = byte(dataType)
	binary.LittleEndian.PutUint16(result[2:4], uint16(ne.ConnectionId.ID))

	switch dataType {
	case NetEventDataTypeByteArray:
		binary.LittleEndian.PutUint32(result[4:8], uint32(len(ne.Data.ObjectData)))
		for i := 0; i < len(ne.Data.ObjectData); i++ {
			result[8+i] = ne.Data.ObjectData[i]
		}
	case NetEventDataTypeUTF16String:
		vals := []rune(*ne.Data.StringData)
		binary.LittleEndian.PutUint32(result[4:8], uint32(len(vals)))

		for i := 0; i < len(vals); i++ {
			binary.LittleEndian.PutUint16(result[8+i*2:8+i*2+2], uint16(vals[i]))
		}
	}

	return result
}
