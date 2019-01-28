package onionbuffer

import (
	"archive/zip"
	"bytes"
	"testing"
)

//func TestWriteFilesToBuffers(t *testing.T) {
//	zb := new(bytes.Buffer)
//	zw := zip.NewWriter(zb)
//	defer zw.Close()
//	var wg sync.WaitGroup
//	//wg.Add(1)
//	uploadQueue := make(chan *multipart.FileHeader, 5)
//
//	go func() {
//		if err := WriteFilesToBuffers(zw, uploadQueue, wg, 1024); err != nil {
//			t.Error(err)
//		}
//	}()
//}

func TestWriteBytesInChunks(t *testing.T) {
	reader := bytes.NewReader([]byte("Test file"))
	zb := new(bytes.Buffer)
	zw := zip.NewWriter(zb)
	defer zw.Close()
	// Create file in zip
	zBuffer, _ := zw.Create("testzip1")

	if err := writeBytesByChunk(reader, zBuffer, 1024); err != nil {
		t.Error(err)
	}
}

func BenchmarkWriteBytesInChunks(b *testing.B) {
	reader := bytes.NewReader([]byte("Test file"))
	zb := new(bytes.Buffer)
	zw := zip.NewWriter(zb)
	defer zw.Close()
	// Create file in zip with same name
	zBuffer, err := zw.Create("testzip2")
	if err != nil {
		b.Error(err)
	}
	for n := 0; n < b.N; n++ {
		writeBytesByChunk(reader, zBuffer, 1024)
	}
}
