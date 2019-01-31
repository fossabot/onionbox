package onionbuffer

import "testing"

func TestGetChecksum(t *testing.T) {
	b := OnionBuffer{Bytes: []byte("Testing checksum"), ChunkSize: 1024}
	if _, err := b.GetChecksum(); err != nil {
		t.Error(err)
	}
}

func BenchmarkGetChecksum(b *testing.B) {
	ob := OnionBuffer{Bytes: []byte("Testing checksum"), ChunkSize: 1024}
	for n := 0; n < b.N; n++ {
		ob.GetChecksum()
	}
}

func TestValidateChecksum(t *testing.T) {
	b := OnionBuffer{Bytes: []byte("Testing checksum"), ChunkSize: 1024}
	b.Checksum, _ = b.GetChecksum()
	validChksm, err := b.ValidateChecksum()
	if err != nil {
		t.Error(err)
	}
	if !validChksm {
		t.Error("Expected checksum to be valid")
	}
}

func BenchmarkValidateChecksum(b *testing.B) {
	ob := OnionBuffer{Bytes: []byte("Testing checksum"), ChunkSize: 1024}
	ob.Checksum, _ = ob.GetChecksum()
	for n := 0; n < b.N; n++ {
		ob.ValidateChecksum()
	}
}
