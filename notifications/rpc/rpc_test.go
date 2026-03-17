package rpc

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"io"
	"sync"
	"testing"
)

func TestRPC(t *testing.T) {
	writer, reader := dummyRemote()
	rpc := NewRPC(writer, reader, zaptest.NewLogger(t).Sugar())

	wg := sync.WaitGroup{}
	for i := range 5 {
		wg.Add(1)
		go func(i int) {
			for j := range 100 {
				params := fmt.Sprintf(`{"go":"%d-%d"}`, i, j)

				res, err := rpc.Call("hello", json.RawMessage(params))
				if err != nil {
					panic(err)
				}

				t.Log(string(res))
				assert.Equal(t, params, string(res))
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func dummyRemote() (io.WriteCloser, io.Reader) {
	reqReader, reqWriter := io.Pipe()
	resReader, resWriter := io.Pipe()

	go func() {
		dec := json.NewDecoder(reqReader)
		enc := json.NewEncoder(resWriter)

		for {
			var req Request
			err := dec.Decode(&req)
			if err != nil {
				panic(err)
			}

			var res Response

			res.Id = req.Id
			res.Result = req.Params

			err = enc.Encode(&res)
			if err != nil {
				panic(err)
			}
		}
	}()

	return reqWriter, resReader
}
