package adbbot

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io/ioutil"
	"testing"
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

func init() {
	// init test data
	for i := 1; i <= 8; i += 1 {
		f := fmt.Sprintf("testdata/%03d.png", i)
		err := read2Cache(f)
		if err != nil {
			fmt.Println("load test data error: ", err)
			panic(err)
		}
	}
}

func checkFrame(a image.Image, b image.Image) bool {
	ta, ok := a.(*image.NRGBA)
	if !ok {
		return false
	}
	tb, ok := a.(*image.NRGBA)
	if !ok {
		return false
	}

	if !ta.Rect.Eq(tb.Rect) {
		return false
	}

	if ta.Stride != tb.Stride {
		return false
	}

	return bytes.Equal(ta.Pix, ta.Pix)
}

func checkFrames(input [][]byte) error {
	if len(input) != len(cachedFrame) {
		return errors.New("Frame count different!!")
	}

	decomp := NewDiffImgDeComp()
	for i, imgByte := range input {
		decode, err := decomp.Decode(imgByte)
		if err != nil {
			return err
		}
		if !checkFrame(cachedFrame[i], decode) {
			return errors.New("Frame different!!")
		}
	}
	return nil
}

func TestPFrame(t *testing.T) {
	decomp := NewDiffImgDeComp()

	buf := bytes.NewBuffer(nil)
	enc := NewDiffImgComp(buf, 8)
	for _, img := range cachedFrame {
		imgByte, err := enc.Encode(img, false)
		if err != nil {
			t.Fatal(err)
			return
		}

		decode, err := decomp.Decode(imgByte)
		if err != nil {
			t.Fatal(err)
			return
		}
		if !checkFrame(img, decode) {
			t.Fatal("Frame different!!")
		}
	}
}

func TestPFrame2(t *testing.T) {
	decomp := NewDiffImgDeComp()

	buf := bytes.NewBuffer(nil)
	enc := NewDiffImgComp(buf, 8)
	for _, img := range cachedFrame {
		imgByte, err := enc.Encode2(img, false)
		if err != nil {
			t.Fatal(err)
			return
		}

		decode, err := decomp.Decode2(imgByte)
		if err != nil {
			t.Fatal(err)
			return
		}
		if !checkFrame(img, decode) {
			t.Fatal("Frame different!!")
		}
	}
}
