package adbbot

import (
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
	Vf(4, info + " took %s", time.Since(timeT0))
}

func NewTmpl(filename string, reg image.Rectangle) (*Tmpl, error) {

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

func LoadTmpl(filename string, reg image.Rectangle) (*Tmpl) {
	tmpl, _ := NewTmpl(filename, reg)
	return tmpl
}

func (t *Tmpl) Center(x, y int) (image.Point) {

	bb := t.Image.Bounds()
	x = bb.Dx() / 2 + x
	y = bb.Dy() / 2 + y

	return image.Pt(x, y)
}


func Rect(x, y, xp, yp int) (image.Rectangle){
	return image.Rect(x, y, x+xp, y+yp)
}

func RectAbs(x, y, x2, y2 int) (image.Rectangle){
	return image.Rect(x, y, x2, y2)
}




// belows are copy and modify form Grigory Dryapak's Imaging
// https://github.com/disintegration/imaging
/*
The MIT License (MIT)

Copyright (c) 2012-2018 Grigory Dryapak

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
	src := newScanner(img)
	dst := image.NewNRGBA(image.Rect(0, 0, src.w, src.h))
	size := src.w * 4
	parallel(0, src.h, func(ys <-chan int) {
		for y := range ys {
			i := y * dst.Stride
			src.scan(0, y, src.w, y+1, dst.Pix[i:i+size])
		}
	})
	return dst
}

func toNRGBA(img image.Image) *image.NRGBA {
	if img, ok := img.(*image.NRGBA); ok {
		return &image.NRGBA{
			Pix:    img.Pix,
			Stride: img.Stride,
			Rect:   img.Rect.Sub(img.Rect.Min),
		}
	}
	return Clone(img)
}

// parallel processes the data in separate goroutines.
func parallel(start, stop int, fn func(<-chan int)) {
	wg := parallelSpawn(start, stop, fn)
	if wg != nil {
		wg.Wait()
	}
}

func parallelSpawn(start, stop int, fn func(<-chan int)) (*sync.WaitGroup) {
	count := stop - start
	if count < 1 {
		return nil
	}

	procs := runtime.GOMAXPROCS(0)
	if procs > count {
		procs = count
	}


	c := make(chan int, procs)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := start; i < stop; i++ {
			c <- i
		}
		close(c)
		wg.Done()
	}()

	for i := 0; i < procs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(c)
		}()
	}
	return &wg
}

