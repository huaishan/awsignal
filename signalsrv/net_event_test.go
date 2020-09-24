package signalsrv

import (
	"bytes"
	"testing"
)

func TestNewNetworkEvent(t *testing.T) {
	s := "123"
	ne := NewNetworkEvent(
		NetEventTypeServerInitialized,
		INVALIDConnectionId,
		&NetEventData{Type: NetEventDataTypeUTF16String, StringData: &s},
	)

	if want, got := NetEventTypeServerInitialized, ne.Type; want != got {
		t.Errorf("expected event type %d got: %d", want, got)
	}

	if want, got := int16(-1), ne.ConnectionId.ID; want != got {
		t.Errorf("expected connectionId %d got: %d", want, got)
	}

	if want, got := NetEventDataTypeUTF16String, ne.Data.Type; want != got {
		t.Errorf("expected data type %s got: %s", want, got)
	}

	if want, got := "123", *ne.Data.StringData; want != got {
		t.Errorf("expected data %s got: %s", want, got)
	}
}

func TestParseFromString(t *testing.T) {
	ne := ParseFromString(`{"type":0,"connectionId":{"id":1}}`)

	if ne == nil {
		t.Errorf("expected ParseFromString want: not nil got: nil")
	}

	if want, got := NetEventTypeInvalid, ne.Type; want != got {
		t.Errorf("expected event type %d got: %d", want, got)
	}

	if want, got := int16(1), ne.ConnectionId.ID; want != got {
		t.Errorf("expected connectionId %d got: %d", want, got)
	}

	if want, got := NetEventDataTypeNull, ne.Data.Type; want != got {
		t.Errorf("expected data type %s got: %s", want, got)
	}

	ne = ParseFromString(`{"type":1,"connectionId":{"id":3},"data":[49,50,51]}`)

	if ne == nil {
		t.Errorf("expected ParseFromString want: not nil got: nil")
	}

	if want, got := NetEventDataTypeByteArray, ne.Data.Type; want != got {
		t.Errorf("expected data type ByteArray %s got: %s", want, got)
	}

	if want, got := []uint8{49, 50, 51}, ne.Data.ObjectData; bytes.Compare(want, got) != 0 {
		t.Errorf("expected data %v got: %v", want, got)
	}

	ne = ParseFromString(`{"type":2,"connectionId":{"id":2},"data":"abc"}`)

	if ne == nil {
		t.Errorf("expected ParseFromString want: not nil got: nil")
	}

	if want, got := NetEventDataTypeUTF16String, ne.Data.Type; want != got {
		t.Errorf("expected type UTF16String %s got: %s", want, got)
	}

	if want, got := "abc", *ne.Data.StringData; want != got {
		t.Errorf("expected data %s got: %s", want, got)
	}
}

func TestFromByteArray(t *testing.T) {
	ne, _ := FromByteArray([]byte{3, 2, 255, 255, 3, 0, 0, 0, 49, 0, 50, 0, 51, 0})

	if want, got := NetEventTypeServerInitialized, ne.Type; want != got {
		t.Errorf("expected event type %d got: %d", want, got)
	}

	if want, got := int16(-1), ne.ConnectionId.ID; want != got {
		t.Errorf("expected connectionId %d got: %d", want, got)
	}

	if want, got := NetEventDataTypeUTF16String, ne.Data.Type; want != got {
		t.Errorf("expected data type %s got: %s", want, got)
	}

	if want, got := "123", *ne.Data.StringData; want != got {
		t.Errorf("expected data %s got: %s", want, got)
	}
}

func TestToByteArray(t *testing.T) {
	s := "123"
	ne := NewNetworkEvent(
		NetEventTypeServerInitialized,
		INVALIDConnectionId,
		&NetEventData{Type: NetEventDataTypeUTF16String, StringData: &s},
	)

	want := []byte{3, 2, 255, 255, 3, 0, 0, 0, 49, 0, 50, 0, 51, 0}
	got := ne.ToByteArray()

	if bytes.Compare(want, got) != 0 {
		t.Errorf("expected to by array want: %v got: %v", want, got)
	}
}
