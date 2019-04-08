package adbbot

import (
	"bytes"
	"encoding/binary"
	"io"
	"image"

	"github.com/golang/snappy"
)


func cp(a []byte) []byte {
	b := make([]byte, len(a), len(a))
	copy(b, a)
	return b
}

type DiffImgComp struct {
	buf *bytes.Buffer
	ref *image.NRGBA
	c int
	N int
	w *snappy.Writer
}

func NewDiffImgComp(buf *bytes.Buffer, n int) *DiffImgComp {
	o := &DiffImgComp{}
	o.buf = buf
	o.w = snappy.NewBufferedWriter(o.buf)
	o.N = n
	return o
}

func (o *DiffImgComp) Encode(img image.Image, foreIframe bool) ([]byte, error) {
	o.buf.Reset()

	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		return nil, ErrUnsupportedFormat
	}

	if o.ref == nil || foreIframe {
		goto FAILBACK
	}

	/*if o.c == 0 {
		o.c = o.N // I + P * o.N
		goto FAILBACK
	}
	o.c -= 1*/

	{
		dX := img.Bounds().Dx()
		dY := img.Bounds().Dy()
		maxSize := dX * dY * 3

		pFrame(nrgba, o.ref, o.w)
		o.w.Flush()
		o.ref = nrgba
		if o.buf.Len() > maxSize {
			Vln(4, "[DiffImg]bigger!", o.buf.Len(), maxSize)
			o.buf.Reset()
			iFrame(nrgba, o.w)
			o.w.Flush()
		}
		return o.buf.Bytes(), nil
	}

FAILBACK:
	iFrame(nrgba, o.w)
	o.w.Flush()
	o.ref = nrgba
	return o.buf.Bytes(), nil
}

func (o *DiffImgComp) Encode2(img image.Image, foreIframe bool) ([]byte, error) {
	o.buf.Reset()

	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		return nil, ErrUnsupportedFormat
	}

	if o.ref == nil || foreIframe {
		goto FAILBACK
	}

	/*if o.c == 0 {
		o.c = o.N // I + P * o.N
		goto FAILBACK
	}
	o.c -= 1*/

	{
		pFrame(nrgba, o.ref, o.buf)
		o.ref = nrgba
		return o.buf.Bytes(), nil
	}

FAILBACK:
	iFrame(nrgba, o.buf)
	o.ref = nrgba
	return o.buf.Bytes(), nil
}

func pFrame(img *image.NRGBA, refimg *image.NRGBA, buf io.Writer) {
	dX := img.Bounds().Dx()
	dY := img.Bounds().Dy()

	// BCE
	bound1 := img.PixOffset(0, 0)
	bound2 := img.PixOffset(dX-1, dY-1) + 3
	_ = img.Pix[bound1:bound2]

	bound1 = refimg.PixOffset(0, 0)
	bound2 = refimg.PixOffset(dX-1, dY-1) + 3
	_ = refimg.Pix[bound1:bound2]

	over := make([]byte, 10, 10)
	WriteVLen := func (conn io.Writer, n int64) {
		overlen := binary.PutVarint(over, int64(n))
		conn.Write(over[:overlen])
	}

	tmp := make([]int16, dX * dY * 3, dX * dY * 3)
	for i := 0; i < dY; i++ {
		for j := 0; j < dX; j++ {
			oi := img.PixOffset(j, i)
			si := refimg.PixOffset(j, i)
			ti := (i * dX + j) * 3

			//r, g, b := int16(refimg.Pix[si + 0]) - int16(img.Pix[oi + 0]), int16(refimg.Pix[si + 1]) - int16(img.Pix[oi + 1]), int16(refimg.Pix[si + 2]) - int16(img.Pix[oi + 2])
			//tmp[ti + 0], tmp[ti + 1], tmp[ti + 2] = r, g, b

			tmp[ti + 0] = int16(refimg.Pix[si + 0]) - int16(img.Pix[oi + 0])
			tmp[ti + 1] = int16(refimg.Pix[si + 1]) - int16(img.Pix[oi + 1])
			tmp[ti + 2] = int16(refimg.Pix[si + 2]) - int16(img.Pix[oi + 2])
		}
	}

	tmp2 := make([]byte, dX * dY * 3 * 2, dX * dY * 3 * 2)
	offset := 0
	for _, v := range tmp {
		overlen := binary.PutVarint(tmp2[offset:], int64(v))
		offset += overlen
	}

	buf.Write([]byte("P"))
	WriteVLen(buf, int64(dX))
	WriteVLen(buf, int64(dY))
	buf.Write(tmp2[:offset])
//	Vln(6, "[DiffImg]P", len(img.Pix), dX, dY, len(tmp), offset, img.Stride, img.Rect, refimg.Stride, refimg.Rect)
}

func iFrame(img *image.NRGBA, buf io.Writer) {
	dX := img.Bounds().Dx()
	dY := img.Bounds().Dy()

	// BCE
	bound1 := img.PixOffset(0, 0)
	bound2 := img.PixOffset(dX-1, dY-1) + 3
	_ = img.Pix[bound1:bound2]

	over := make([]byte, 10, 10)
	WriteVLen := func (conn io.Writer, n int64) {
		overlen := binary.PutVarint(over, int64(n))
		conn.Write(over[:overlen])
	}

	buf.Write([]byte("R"))
	WriteVLen(buf, int64(dX))
	WriteVLen(buf, int64(dY))

	tmp := make([]byte, dX * dY * 3, dX * dY * 3)
	ti := 0
	for i := 0; i < dY; i++ {
		for j := 0; j < dX; j++ {
			si := img.PixOffset(j, i)
			tmp[ti + 0], tmp[ti + 1], tmp[ti + 2] = img.Pix[si + 0], img.Pix[si + 1], img.Pix[si + 2]
			ti += 3
		}
	}
	buf.Write(tmp[:ti])
}

type DiffImgDeComp struct {
	buf *bytes.Buffer
	ref *image.NRGBA
	r *snappy.Reader
	//r io.Reader
}

func NewDiffImgDeComp() *DiffImgDeComp {
	o := &DiffImgDeComp{}
	o.buf = bytes.NewBuffer(nil)
	o.r = snappy.NewReader(o.buf)
	//o.r = o.buf
	return o
}

func (o *DiffImgDeComp) Decode(imgByte []byte) (image.Image, error) {
	return o.decode(imgByte, o.r)
}

func (o *DiffImgDeComp) Decode2(imgByte []byte) (image.Image, error) {
	return o.decode(imgByte, o.buf)
}

func (o *DiffImgDeComp) decode(imgByte []byte, r io.Reader) (image.Image, error) {
	o.buf.Write(imgByte)

	frameType := make([]byte, 1, 1)
	r.Read(frameType)
	dX, err := ReadVLen(r)
	if err != nil {
		return nil, err
	}
	dY, err := ReadVLen(r)
	if err != nil {
		return nil, err
	}

//	Vln(4, "[DiffImgDeComp]Decode!", frameType[0], dX, dY)
	nrgba := image.NewNRGBA(image.Rect(0, 0, int(dX), int(dY)))
	switch frameType[0] {
	case 'R':
		pxCount := dX * dY
		_, err := io.ReadFull(r, nrgba.Pix[:pxCount * 3])
		if err != nil {
			return nil, err
		}
		iFrameParse(nrgba, int(dX), int(dY))

	case 'P':
//		Vln(5, "[DiffImgDeComp]P-buf", o.buf.Len())
		pFrameParse(r, nrgba, int(dX), int(dY), o.ref)
	}

	o.ref = nrgba
	o.buf.Reset()
	return nrgba, nil
}
func iFrameParse(img *image.NRGBA, dX int, dY int) {
	for i := int(dY - 1); i >= 0; i-- {
		for j := int(dX - 1); j >= 0; j-- {
			si := img.PixOffset(j, i)
			ti := (i * dX + j) * 3
			img.Pix[si + 0], img.Pix[si + 1], img.Pix[si + 2], img.Pix[si + 3] = img.Pix[ti + 0], img.Pix[ti + 1], img.Pix[ti + 2], 0xff
		}
	}
}
func pFrameParse(pixIn io.Reader, img *image.NRGBA, dX int, dY int, ref *image.NRGBA) {
	reader := &byteReader{pixIn}
	ReadVLen := func () (int, error){
		t, err := binary.ReadVarint(reader)
		return int(t), err
	}

	getRefPx := func (ri int) (int, int, int){
		return int(ref.Pix[ri + 0]), int(ref.Pix[ri + 1]), int(ref.Pix[ri + 2])
	}

	for i := 0; i < dY; i++ {
		for j := 0; j < dX; j++ {
			si := img.PixOffset(j, i)
			r, g, b := getRefPx(si)

			t, err := ReadVLen()
			if err != nil {
				return
			}
			r = r - t

			t, err = ReadVLen()
			if err != nil {
				return
			}
			g = g - t

			t, err = ReadVLen()
			if err != nil {
				return
			}
			b = b - t

			img.Pix[si + 0], img.Pix[si + 1], img.Pix[si + 2], img.Pix[si + 3] = uint8(r), uint8(g), uint8(b), 0xff
		}
	}
}

