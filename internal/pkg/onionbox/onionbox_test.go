package onionbox

import "testing"

//func BenchmarkWriteFilesToBuffers(b *testing.B) {
//	ob := &onionbox{Store: onion_buffer.NewStore()}
//	zb := new(bytes.Buffer)
//	zw := zip.NewWriter(zb)
//	defer zw.Close()
//	var wg sync.WaitGroup
//	wg.Add(0)
//	uploadQueue := make(chan *multipart.FileHeader, 10000)
//
//	testDir, _ := ioutil.ReadDir("tests")
//	for _, file := range testDir {
//		fh := &multipart.FileHeader{Filename: fmt.Sprintf("./tests/%s", file.Name())}
//		uploadQueue <- fh
//	}
//
//	for n := 0; n < b.N; n++ {
//		ob.writeFilesToBuffers(zw, uploadQueue, &wg)
//	}
//}

func TestCreateCSRF(t *testing.T) {
	_, err := createCSRF()
	if err != nil {
		t.Error(err)
	}
}
