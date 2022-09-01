// run: go run .
package main

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"reflect"
	"runtime"
	"testing"

	"image/jpeg"
	"image/png"

	. ".."

	"compress/flate"

	"github.com/golang/snappy"
)

var (
	cachedFrame     = make([]image.Image, 0)
	cachedFrameByte = make([][]byte, 0)
)

func read2Cache(file string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	img, err := Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}

	cachedFrameByte = append(cachedFrameByte, data)
	cachedFrame = append(cachedFrame, img)

	return nil
}

func cp(a []byte) []byte {
	b := make([]byte, len(a), len(a))
	copy(b, a)
	return b
}

func init() {
	// init test data
	for i := 1; i <= 8; i += 1 {
		f := fmt.Sprintf("%03d.png", i)
		err := read2Cache(f)
		if err != nil {
			fmt.Println("load test data error: ", err)
			panic(err)
		}
	}
}

func code2Jpg(queue chan []byte) {
	buf := bytes.NewBuffer(nil)
	for _, img := range cachedFrame {
		buf.Reset()
		jpeg.Encode(buf, img, &jpeg.Options{100})
		queue <- cp(buf.Bytes())
	}
	close(queue)
	return
}

func code2Png(queue chan []byte) {
	encoder := png.Encoder{}

	buf := bytes.NewBuffer(nil)
	for _, img := range cachedFrame {
		buf.Reset()
		encoder.Encode(buf, img)
		queue <- cp(buf.Bytes())
	}
	close(queue)
	return
}

func code2PngNoCompression(queue chan []byte) {
	encoder := png.Encoder{
		CompressionLevel: png.NoCompression,
	}

	buf := bytes.NewBuffer(nil)
	for _, img := range cachedFrame {
		buf.Reset()
		encoder.Encode(buf, img)
		queue <- cp(buf.Bytes())
	}
	close(queue)
	return
}

func code2PngBestSpeed(queue chan []byte) {
	encoder := png.Encoder{
		CompressionLevel: png.BestSpeed,
	}

	buf := bytes.NewBuffer(nil)
	for _, img := range cachedFrame {
		buf.Reset()
		encoder.Encode(buf, img)
		queue <- cp(buf.Bytes())
	}
	close(queue)
	return
}

func code2Diff(queue chan []byte) {
	buf := bytes.NewBuffer(nil)
	enc := NewDiffImgComp(buf, 8)
	for _, img := range cachedFrame {
		d, _ := enc.Encode2(img, false)
		queue <- cp(d)
	}
	close(queue)
	return
}
func code2DiffC(queue chan []byte) {
	buf := bytes.NewBuffer(nil)
	enc := NewDiffImgComp(buf, 8)
	for _, img := range cachedFrame {
		d, _ := enc.Encode(img, false)
		queue <- cp(d)
	}
	close(queue)
	return
}

func code2OnlyI(queue chan []byte) {
	buf := bytes.NewBuffer(nil)
	enc := NewDiffImgComp(buf, 8)
	for _, img := range cachedFrame {
		d, _ := enc.Encode2(img, true)
		queue <- cp(d)
	}
	close(queue)
	return
}
func code2OnlyIC(queue chan []byte) {
	buf := bytes.NewBuffer(nil)
	enc := NewDiffImgComp(buf, 8)
	for _, img := range cachedFrame {
		d, _ := enc.Encode(img, true)
		queue <- cp(d)
	}
	close(queue)
	return
}

func testPipeByte(fn func(chan []byte), b io.Writer) {
	ch := make(chan []byte)
	go fn(ch)
	for imgByte := range ch {
		b.Write(imgByte)
	}
}

func getOutput(benchFunc func(chan []byte), pipeFunc func(io.Writer) io.Writer) []byte {
	buf := bytes.NewBuffer(nil)
	conn := pipeFunc(buf)
	buf.Reset()
	testPipeByte(benchFunc, conn)
	return buf.Bytes()
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func runBench(benchFunc func(chan []byte), pipeFunc func(io.Writer) io.Writer) {
	fn := func(b *testing.B) {
		buf := bytes.NewBuffer(nil)
		conn := pipeFunc(buf)
		for n := 0; n < b.N; n++ {
			buf.Reset()
			testPipeByte(benchFunc, conn)
		}
	}

	fnSize := func() int {
		buf := bytes.NewBuffer(nil)
		conn := pipeFunc(buf)
		buf.Reset()
		testPipeByte(benchFunc, conn)
		return buf.Len()
	}

	funcName := getFunctionName(benchFunc)
	//size := fnSize() / len(cachedFrameByte)
	size := fnSize()
	br := testing.Benchmark(fn)
	fmt.Printf("%-24s\t%d\t%d ms/op\t%v\t%d bytes\t%s\n", funcName, br.N, br.NsPerOp()/1e6, br.MemString(), size, Vsize(size))
}

func main() {

	println("-- img count =", len(cachedFrame))

	println("-- no Compression stream --")
	runBench(code2Diff, nopStream)
	runBench(code2DiffC, nopStream)
	runBench(code2OnlyI, nopStream)
	runBench(code2OnlyIC, nopStream)

	runBench(code2Jpg, nopStream)
	runBench(code2Png, nopStream)
	runBench(code2PngBestSpeed, nopStream)
	runBench(code2PngNoCompression, nopStream)

	println("-- snappy Stream --")
	runBench(code2Diff, newSnappyStream)
	runBench(code2DiffC, newSnappyStream)
	runBench(code2OnlyI, newSnappyStream)
	runBench(code2OnlyIC, newSnappyStream)

	runBench(code2Jpg, newSnappyStream)
	// runBench(code2Png, newSnappyStream)
	runBench(code2PngBestSpeed, newSnappyStream)
	runBench(code2PngNoCompression, newSnappyStream)

	println("-- flate Stream --")
	runBench(code2Diff, newFlateStream)
	runBench(code2DiffC, newFlateStream)
	runBench(code2OnlyI, newFlateStream)
	runBench(code2OnlyIC, newFlateStream)

	runBench(code2Jpg, newFlateStream)
	runBench(code2Png, newFlateStream)
	runBench(code2PngBestSpeed, newFlateStream)
	runBench(code2PngNoCompression, newFlateStream)

}

func Vsize(bytes int) (ret string) {
	var tmp float64 = float64(bytes)
	var s string = " "

	switch {
	case bytes < (2 << 9):

	case bytes < (2 << 19):
		tmp = tmp / float64(2<<9)
		s = "K"

	case bytes < (2 << 29):
		tmp = tmp / float64(2<<19)
		s = "M"

	case bytes < (2 << 39):
		tmp = tmp / float64(2<<29)
		s = "G"

	case bytes < (2 << 49):
		tmp = tmp / float64(2<<39)
		s = "T"

	}
	ret = fmt.Sprintf("%.3f %sB", tmp, s)
	return
}

func nopStream(conn io.Writer) io.Writer {
	return conn
}

type snappyStream struct {
	w *snappy.Writer
}

func (c *snappyStream) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	err = c.w.Flush()
	return n, err
}
func newSnappyStream(conn io.Writer) io.Writer {
	c := &snappyStream{snappy.NewBufferedWriter(conn)}
	return c
}

type flateStream struct {
	w *flate.Writer
}

func (c *flateStream) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	err = c.w.Flush()
	return n, err
}
func newFlateStream(conn io.Writer) io.Writer {
	c := &flateStream{}
	c.w, _ = flate.NewWriter(conn, flate.BestSpeed)
	// c.w, _ = flate.NewWriter(conn, flate.HuffmanOnly)
	return c
}
