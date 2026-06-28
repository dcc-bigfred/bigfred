package withrottle

import (
	"io"
	"net"
	"sync"
	"testing"
)

func TestWriteLineConcurrent(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go io.Copy(io.Discard, client)

	w := NewWireState()
	w.SetConn("k", server)

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.WriteLine("k", "M0AS3<;>V0")
		}()
	}
	wg.Wait()
}
