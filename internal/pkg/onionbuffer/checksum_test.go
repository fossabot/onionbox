package onionbuffer

import "testing"

func TestGetChecksum(t *testing.T) {
	b := OnionBuffer{Bytes: []byte("Testing checksum")}
	if _, err := b.GetChecksum(); err != nil {
		t.Error(err)
	}
}

func TestValidateChecksum(t *testing.T) {
	b := OnionBuffer{Bytes: []byte("Testing checksum")}
	b.Checksum, _ = b.GetChecksum()
	validChksm, err := b.ValidateChecksum()
	if err != nil {
		t.Error(err)
	}
	if !validChksm {
		t.Error("Expected checksum to be valid")
	}
}
