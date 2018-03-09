// build for linux amd64: GOOS=linux GOARCH=amd64 go test -c -o test.x64
// build for linux arm7:  GOOS=linux GOARCH=arm GOARM=7 go test -c -o test.arm7
// run: ./test.x64 -test.bench=. -test.benchmem -test.cpu=1,2,4,8
package adbbot

import (
	"image"
	"testing"
)

var tmpl image.Image
var input image.Image

func init() {
	input, _ = OpenImage("testdata/input.png")
	tmpl, _ = OpenImage("testdata/tmpl.png")
}

func BenchmarkFind(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Find(input, tmpl)
	}
}

func BenchmarkFindP(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FindP(input, tmpl)
	}
}

func BenchmarkFindP2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FindP2(input, tmpl)
	}
}


