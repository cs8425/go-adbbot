package adbbot

import (
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var timeT0 time.Time

type Tmpl struct {
	Image      image.Image
	Region     image.Rectangle
	ImagePath  string
}

var RectAll = image.ZR

func timeStart() (){
	timeT0 = time.Now()
}

func timeEnd(info string) (){
	Vf(3, info + " took %s", time.Since(timeT0))
}

func NewTmpl(filename string, reg image.Rectangle) (*Tmpl, error){

	img, err:= OpenImage(filename)
	if err != nil {
		return nil, err
	}

	tmpl := Tmpl{
		Image:      img,
		Region:     reg,
		ImagePath:  filename,
	}

	return &tmpl, nil
}

func NewRect(x, y, xp, yp int) (image.Rectangle){
	return image.Rect(x, y, x+xp, y+yp)
}

func NewRectAbs(x, y, x2, y2 int) (image.Rectangle){
	return image.Rect(x, y, x2, y2)
}

func NewRectAll() (image.Rectangle){
	return image.ZR
}



// belows are copy and modify form Grigory Dryapak's Imaging
// https://github.com/disintegration/imaging
/*
The MIT License (MIT)

Copyright (c) 2012-2014 Grigory Dryapak

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
type Format int

const (
	JPEG Format = iota
	PNG
	GIF
)

var (
	ErrUnsupportedFormat = errors.New("imaging: unsupported image format")
)


// Open loads an image from file
func OpenImage(filename string) (image.Image, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, err := Decode(file)
	return img, err
}

// Decode reads an image from r.
func Decode(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	if src0, ok := img.(*image.NRGBA); ok {
		return src0, nil
	}
	return Clone(img), nil
}

// Save saves the image to file with the specified filename.
// The format is determined from the filename extension: "jpg" (or "jpeg"), "png", "gif" are supported.
func SaveImage(img image.Image, filename string) (err error) {
	formats := map[string]Format{
		".jpg":  JPEG,
		".jpeg": JPEG,
		".png":  PNG,
		".gif":  GIF,
	}

	ext := strings.ToLower(filepath.Ext(filename))
	f, ok := formats[ext]
	if !ok {
		return ErrUnsupportedFormat
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return Encode(file, img, f)
}

// Encode writes the image img to w in the specified format (JPEG, PNG, GIF, TIFF or BMP).
func Encode(w io.Writer, img image.Image, format Format) error {
	var err error
	switch format {
	case JPEG:
		var rgba *image.RGBA
		if nrgba, ok := img.(*image.NRGBA); ok {
			if nrgba.Opaque() {
				rgba = &image.RGBA{
					Pix:    nrgba.Pix,
					Stride: nrgba.Stride,
					Rect:   nrgba.Rect,
				}
			}
		}
		if rgba != nil {
			err = jpeg.Encode(w, rgba, &jpeg.Options{Quality: 95})
		} else {
			err = jpeg.Encode(w, img, &jpeg.Options{Quality: 95})
		}

	case PNG:
		err = png.Encode(w, img)
	case GIF:
		err = gif.Encode(w, img, &gif.Options{NumColors: 256})
	default:
		err = ErrUnsupportedFormat
	}
	return err
}

// Clone returns a copy of the given image.
func Clone(img image.Image) *image.NRGBA {
	srcBounds := img.Bounds()
	srcMinX := srcBounds.Min.X
	srcMinY := srcBounds.Min.Y

	dstBounds := srcBounds.Sub(srcBounds.Min)
	dstW := dstBounds.Dx()
	dstH := dstBounds.Dy()
	dst := image.NewNRGBA(dstBounds)

	switch src := img.(type) {

	case *image.NRGBA:
		rowSize := srcBounds.Dx() * 4
		parallel(dstH, 1, func(partStart, partEnd int) {
			for dstY := partStart; dstY < partEnd; dstY++ {
				di := dst.PixOffset(0, dstY)
				si := src.PixOffset(srcMinX, srcMinY+dstY)
				copy(dst.Pix[di:di+rowSize], src.Pix[si:si+rowSize])
			}
		})

	case *image.NRGBA64:
		parallel(dstH, 1, func(partStart, partEnd int) {
			for dstY := partStart; dstY < partEnd; dstY++ {
				di := dst.PixOffset(0, dstY)
				si := src.PixOffset(srcMinX, srcMinY+dstY)
				for dstX := 0; dstX < dstW; dstX++ {

					dst.Pix[di+0] = src.Pix[si+0]
					dst.Pix[di+1] = src.Pix[si+2]
					dst.Pix[di+2] = src.Pix[si+4]
					dst.Pix[di+3] = src.Pix[si+6]

					di += 4
					si += 8

				}
			}
		})

	case *image.RGBA:
		parallel(dstH, 1, func(partStart, partEnd int) {
			for dstY := partStart; dstY < partEnd; dstY++ {
				di := dst.PixOffset(0, dstY)
				si := src.PixOffset(srcMinX, srcMinY+dstY)
				for dstX := 0; dstX < dstW; dstX++ {

					a := src.Pix[si+3]
					dst.Pix[di+3] = a
					switch a {
					case 0:
						dst.Pix[di+0] = 0
						dst.Pix[di+1] = 0
						dst.Pix[di+2] = 0
					case 0xff:
						dst.Pix[di+0] = src.Pix[si+0]
						dst.Pix[di+1] = src.Pix[si+1]
						dst.Pix[di+2] = src.Pix[si+2]
					default:
						var tmp uint16
						tmp = uint16(src.Pix[si+0]) * 0xff / uint16(a)
						dst.Pix[di+0] = uint8(tmp)
						tmp = uint16(src.Pix[si+1]) * 0xff / uint16(a)
						dst.Pix[di+1] = uint8(tmp)
						tmp = uint16(src.Pix[si+2]) * 0xff / uint16(a)
						dst.Pix[di+2] = uint8(tmp)
					}

					di += 4
					si += 4

				}
			}
		})

	case *image.RGBA64:
		parallel(dstH, 1, func(partStart, partEnd int) {
			for dstY := partStart; dstY < partEnd; dstY++ {
				di := dst.PixOffset(0, dstY)
				si := src.PixOffset(srcMinX, srcMinY+dstY)
				for dstX := 0; dstX < dstW; dstX++ {

					a := src.Pix[si+6]
					dst.Pix[di+3] = a
					switch a {
					case 0:
						dst.Pix[di+0] = 0
						dst.Pix[di+1] = 0
						dst.Pix[di+2] = 0
					case 0xff:
						dst.Pix[di+0] = src.Pix[si+0]
						dst.Pix[di+1] = src.Pix[si+2]
						dst.Pix[di+2] = src.Pix[si+4]
					default:
						var tmp uint16
						tmp = uint16(src.Pix[si+0]) * 0xff / uint16(a)
						dst.Pix[di+0] = uint8(tmp)
						tmp = uint16(src.Pix[si+2]) * 0xff / uint16(a)
						dst.Pix[di+1] = uint8(tmp)
						tmp = uint16(src.Pix[si+4]) * 0xff / uint16(a)
						dst.Pix[di+2] = uint8(tmp)
					}

					di += 4
					si += 8

				}
			}
		})

	case *image.Gray:
		parallel(dstH, 1, func(partStart, partEnd int) {
			for dstY := partStart; dstY < partEnd; dstY++ {
				di := dst.PixOffset(0, dstY)
				si := src.PixOffset(srcMinX, srcMinY+dstY)
				for dstX := 0; dstX < dstW; dstX++ {

					c := src.Pix[si]
					dst.Pix[di+0] = c
					dst.Pix[di+1] = c
					dst.Pix[di+2] = c
					dst.Pix[di+3] = 0xff

					di += 4
					si += 1

				}
			}
		})

	case *image.Gray16:
		parallel(dstH, 1, func(partStart, partEnd int) {
			for dstY := partStart; dstY < partEnd; dstY++ {
				di := dst.PixOffset(0, dstY)
				si := src.PixOffset(srcMinX, srcMinY+dstY)
				for dstX := 0; dstX < dstW; dstX++ {

					c := src.Pix[si]
					dst.Pix[di+0] = c
					dst.Pix[di+1] = c
					dst.Pix[di+2] = c
					dst.Pix[di+3] = 0xff

					di += 4
					si += 2

				}
			}
		})

	case *image.YCbCr:
		parallel(dstH, 1, func(partStart, partEnd int) {
			for dstY := partStart; dstY < partEnd; dstY++ {
				di := dst.PixOffset(0, dstY)
				for dstX := 0; dstX < dstW; dstX++ {

					srcX := srcMinX + dstX
					srcY := srcMinY + dstY
					siy := src.YOffset(srcX, srcY)
					sic := src.COffset(srcX, srcY)
					r, g, b := color.YCbCrToRGB(src.Y[siy], src.Cb[sic], src.Cr[sic])
					dst.Pix[di+0] = r
					dst.Pix[di+1] = g
					dst.Pix[di+2] = b
					dst.Pix[di+3] = 0xff

					di += 4

				}
			}
		})

	case *image.Paletted:
		plen := len(src.Palette)
		pnew := make([]color.NRGBA, plen)
		for i := 0; i < plen; i++ {
			pnew[i] = color.NRGBAModel.Convert(src.Palette[i]).(color.NRGBA)
		}

		parallel(dstH, 1, func(partStart, partEnd int) {
			for dstY := partStart; dstY < partEnd; dstY++ {
				di := dst.PixOffset(0, dstY)
				si := src.PixOffset(srcMinX, srcMinY+dstY)
				for dstX := 0; dstX < dstW; dstX++ {

					c := pnew[src.Pix[si]]
					dst.Pix[di+0] = c.R
					dst.Pix[di+1] = c.G
					dst.Pix[di+2] = c.B
					dst.Pix[di+3] = c.A

					di += 4
					si += 1

				}
			}
		})

	default:
		parallel(dstH, 1, func(partStart, partEnd int) {
			for dstY := partStart; dstY < partEnd; dstY++ {
				di := dst.PixOffset(0, dstY)
				for dstX := 0; dstX < dstW; dstX++ {

					c := color.NRGBAModel.Convert(img.At(srcMinX+dstX, srcMinY+dstY)).(color.NRGBA)
					dst.Pix[di+0] = c.R
					dst.Pix[di+1] = c.G
					dst.Pix[di+2] = c.B
					dst.Pix[di+3] = c.A

					di += 4

				}
			}
		})

	}

	return dst
}

// if GOMAXPROCS = 1: no goroutines used
// if GOMAXPROCS > 1: spawn N=GOMAXPROCS workers in separate goroutines
func parallel(dataSize int, minSize int, fn func(partStart, partEnd int)) {
	numGoroutines := 1
	partSize := dataSize

	numProcs := runtime.GOMAXPROCS(0)
	if numProcs > 1 {
		numGoroutines = numProcs
		partSize = dataSize / (numGoroutines * 16)
		if partSize < minSize {
			partSize = minSize
		}
	}

	if numGoroutines == 1 {
		fn(0, dataSize)
	} else {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		idx := uint64(0)

		for p := 0; p < numGoroutines; p++ {
			go func() {
				defer wg.Done()
				for {
					partStart := int(atomic.AddUint64(&idx, uint64(partSize))) - partSize
					if partStart >= dataSize {
						break
					}
					partEnd := partStart + partSize
					if partEnd > dataSize {
						partEnd = dataSize
					}
					fn(partStart, partEnd)
				}
			}()
		}

		wg.Wait()
	}
}


