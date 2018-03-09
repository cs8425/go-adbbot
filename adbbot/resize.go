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

original source: https://github.com/disintegration/imaging/blob/master/resize.go

*/

package adbbot

import (
	"image"
	"math"
)

// clamp rounds and clamps float64 value to fit into uint8.
func clamp(x float64) uint8 {
	v := int64(x + 0.5)
	if v > 255 {
		return 255
	}
	if v > 0 {
		return uint8(v)
	}
	return 0
}

type indexWeight struct {
	index  int
	weight float64
}

func precomputeWeights(dstSize, srcSize int, filter ResampleFilter) [][]indexWeight {
	du := float64(srcSize) / float64(dstSize)
	scale := du
	if scale < 1.0 {
		scale = 1.0
	}
	ru := math.Ceil(scale * filter.Support)

	out := make([][]indexWeight, dstSize)
	tmp := make([]indexWeight, 0, dstSize*int(ru+2)*2)

	for v := 0; v < dstSize; v++ {
		fu := (float64(v)+0.5)*du - 0.5

		begin := int(math.Ceil(fu - ru))
		if begin < 0 {
			begin = 0
		}
		end := int(math.Floor(fu + ru))
		if end > srcSize-1 {
			end = srcSize - 1
		}

		var sum float64
		for u := begin; u <= end; u++ {
			w := filter.Kernel((float64(u) - fu) / scale)
			if w != 0 {
				sum += w
				tmp = append(tmp, indexWeight{index: u, weight: w})
			}
		}
		if sum != 0 {
			for i := range tmp {
				tmp[i].weight /= sum
			}
		}

		out[v] = tmp
		tmp = tmp[len(tmp):]
	}

	return out
}

// Resize resizes the image to the specified width and height using the specified resampling
// filter and returns the transformed image. If one of width or height is 0, the image aspect
// ratio is preserved.
//
// Supported resample filters: NearestNeighbor, Box, Linear, Hermite, MitchellNetravali,
// CatmullRom, BSpline, Gaussian, Lanczos, Hann, Hamming, Blackman, Bartlett, Welch, Cosine.
//
// Usage example:
//
//	dstImage := imaging.Resize(srcImage, 800, 600, imaging.Lanczos)
//
func Resize(img image.Image, width, height int, filter ResampleFilter) *image.NRGBA {
	dstW, dstH := width, height
	if dstW < 0 || dstH < 0 {
		return &image.NRGBA{}
	}
	if dstW == 0 && dstH == 0 {
		return &image.NRGBA{}
	}

	srcW := img.Bounds().Dx()
	srcH := img.Bounds().Dy()
	if srcW <= 0 || srcH <= 0 {
		return &image.NRGBA{}
	}

	// If new width or height is 0 then preserve aspect ratio, minimum 1px.
	if dstW == 0 {
		tmpW := float64(dstH) * float64(srcW) / float64(srcH)
		dstW = int(math.Max(1.0, math.Floor(tmpW+0.5)))
	}
	if dstH == 0 {
		tmpH := float64(dstW) * float64(srcH) / float64(srcW)
		dstH = int(math.Max(1.0, math.Floor(tmpH+0.5)))
	}

	if filter.Support <= 0 {
		// Nearest-neighbor special case.
		return resizeNearest(img, dstW, dstH)
	}

	if srcW != dstW && srcH != dstH {
		return resizeVertical(resizeHorizontal(img, dstW, filter), dstH, filter)
	}
	if srcW != dstW {
		return resizeHorizontal(img, dstW, filter)
	}
	if srcH != dstH {
		return resizeVertical(img, dstH, filter)
	}
	return Clone(img)
}

func resizeHorizontal(img image.Image, width int, filter ResampleFilter) *image.NRGBA {
	src := newScanner(img)
	dst := image.NewNRGBA(image.Rect(0, 0, width, src.h))
	weights := precomputeWeights(width, src.w, filter)
	parallel(0, src.h, func(ys <-chan int) {
		scanLine := make([]uint8, src.w*4)
		for y := range ys {
			src.scan(0, y, src.w, y+1, scanLine)
			j0 := y * dst.Stride
			for x := 0; x < width; x++ {
				var r, g, b, a float64
				for _, w := range weights[x] {
					i := w.index * 4
					aw := float64(scanLine[i+3]) * w.weight
					r += float64(scanLine[i+0]) * aw
					g += float64(scanLine[i+1]) * aw
					b += float64(scanLine[i+2]) * aw
					a += aw
				}
				if a != 0 {
					aInv := 1 / a
					j := j0 + x*4
					dst.Pix[j+0] = clamp(r * aInv)
					dst.Pix[j+1] = clamp(g * aInv)
					dst.Pix[j+2] = clamp(b * aInv)
					dst.Pix[j+3] = clamp(a)
				}
			}
		}
	})
	return dst
}

func resizeVertical(img image.Image, height int, filter ResampleFilter) *image.NRGBA {
	src := newScanner(img)
	dst := image.NewNRGBA(image.Rect(0, 0, src.w, height))
	weights := precomputeWeights(height, src.h, filter)
	parallel(0, src.w, func(xs <-chan int) {
		scanLine := make([]uint8, src.h*4)
		for x := range xs {
			src.scan(x, 0, x+1, src.h, scanLine)
			for y := 0; y < height; y++ {
				var r, g, b, a float64
				for _, w := range weights[y] {
					i := w.index * 4
					aw := float64(scanLine[i+3]) * w.weight
					r += float64(scanLine[i+0]) * aw
					g += float64(scanLine[i+1]) * aw
					b += float64(scanLine[i+2]) * aw
					a += aw
				}
				if a != 0 {
					aInv := 1 / a
					j := y*dst.Stride + x*4
					dst.Pix[j+0] = clamp(r * aInv)
					dst.Pix[j+1] = clamp(g * aInv)
					dst.Pix[j+2] = clamp(b * aInv)
					dst.Pix[j+3] = clamp(a)
				}
			}
		}
	})
	return dst
}

// resizeNearest is a fast nearest-neighbor resize, no filtering.
func resizeNearest(img image.Image, width, height int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	dx := float64(img.Bounds().Dx()) / float64(width)
	dy := float64(img.Bounds().Dy()) / float64(height)

	if height < img.Bounds().Dy() {
		src := newScanner(img)
		parallel(0, height, func(ys <-chan int) {
			scanLine := make([]uint8, src.w*4)
			for y := range ys {
				srcY := int((float64(y) + 0.5) * dy)
				src.scan(0, srcY, src.w, srcY+1, scanLine)
				dstOff := y * dst.Stride
				for x := 0; x < width; x++ {
					srcX := int((float64(x) + 0.5) * dx)
					srcOff := srcX * 4
					copy(dst.Pix[dstOff:dstOff+4], scanLine[srcOff:srcOff+4])
					dstOff += 4
				}
			}
		})
	} else {
		src := toNRGBA(img)
		parallel(0, height, func(ys <-chan int) {
			for y := range ys {
				srcY := int((float64(y) + 0.5) * dy)
				srcOff0 := srcY * src.Stride
				dstOff := y * dst.Stride
				for x := 0; x < width; x++ {
					srcX := int((float64(x) + 0.5) * dx)
					srcOff := srcOff0 + srcX*4
					copy(dst.Pix[dstOff:dstOff+4], src.Pix[srcOff:srcOff+4])
					dstOff += 4
				}
			}
		})
	}

	return dst
}

// Fit scales down the image using the specified resample filter to fit the specified
// maximum width and height and returns the transformed image.
//
// Supported resample filters: NearestNeighbor, Box, Linear, Hermite, MitchellNetravali,
// CatmullRom, BSpline, Gaussian, Lanczos, Hann, Hamming, Blackman, Bartlett, Welch, Cosine.
//
// Usage example:
//
//	dstImage := imaging.Fit(srcImage, 800, 600, imaging.Lanczos)
//
func Fit(img image.Image, width, height int, filter ResampleFilter) *image.NRGBA {
	maxW, maxH := width, height

	if maxW <= 0 || maxH <= 0 {
		return &image.NRGBA{}
	}

	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	if srcW <= 0 || srcH <= 0 {
		return &image.NRGBA{}
	}

	if srcW <= maxW && srcH <= maxH {
		return Clone(img)
	}

	srcAspectRatio := float64(srcW) / float64(srcH)
	maxAspectRatio := float64(maxW) / float64(maxH)

	var newW, newH int
	if srcAspectRatio > maxAspectRatio {
		newW = maxW
		newH = int(float64(newW) / srcAspectRatio)
	} else {
		newH = maxH
		newW = int(float64(newH) * srcAspectRatio)
	}

	return Resize(img, newW, newH, filter)
}


// ResampleFilter is a resampling filter struct. It can be used to define custom filters.
//
// Supported resample filters: NearestNeighbor, Box, Linear, Hermite, MitchellNetravali,
// CatmullRom, BSpline, Gaussian, Lanczos, Hann, Hamming, Blackman, Bartlett, Welch, Cosine.
//
//	General filter recommendations:
//
//	- Lanczos
//		High-quality resampling filter for photographic images yielding sharp results.
//		It's slower than cubic filters (see below).
//
//	- CatmullRom
//		A sharp cubic filter. It's a good filter for both upscaling and downscaling if sharp results are needed.
//
//	- MitchellNetravali
//		A high quality cubic filter that produces smoother results with less ringing artifacts than CatmullRom.
//
//	- BSpline
//		A good filter if a very smooth output is needed.
//
//	- Linear
//		Bilinear interpolation filter, produces reasonably good, smooth output.
//		It's faster than cubic filters.
//
//	- Box
//		Simple and fast averaging filter appropriate for downscaling.
//		When upscaling it's similar to NearestNeighbor.
//
//	- NearestNeighbor
//		Fastest resampling filter, no antialiasing.
//
type ResampleFilter struct {
	Support float64
	Kernel  func(float64) float64
}

// NearestNeighbor is a nearest-neighbor filter (no anti-aliasing).
var NearestNeighbor ResampleFilter

// Box filter (averaging pixels).
var Box ResampleFilter

// Linear filter.
var Linear ResampleFilter

// Hermite cubic spline filter (BC-spline; B=0; C=0).
var Hermite ResampleFilter

// MitchellNetravali is Mitchell-Netravali cubic filter (BC-spline; B=1/3; C=1/3).
var MitchellNetravali ResampleFilter

// CatmullRom is a Catmull-Rom - sharp cubic filter (BC-spline; B=0; C=0.5).
var CatmullRom ResampleFilter

// BSpline is a smooth cubic filter (BC-spline; B=1; C=0).
var BSpline ResampleFilter

// Gaussian is a Gaussian blurring Filter.
var Gaussian ResampleFilter

// Bartlett is a Bartlett-windowed sinc filter (3 lobes).
var Bartlett ResampleFilter

// Lanczos filter (3 lobes).
var Lanczos ResampleFilter

// Hann is a Hann-windowed sinc filter (3 lobes).
var Hann ResampleFilter

// Hamming is a Hamming-windowed sinc filter (3 lobes).
var Hamming ResampleFilter

// Blackman is a Blackman-windowed sinc filter (3 lobes).
var Blackman ResampleFilter

// Welch is a Welch-windowed sinc filter (parabolic window, 3 lobes).
var Welch ResampleFilter

// Cosine is a Cosine-windowed sinc filter (3 lobes).
var Cosine ResampleFilter

func bcspline(x, b, c float64) float64 {
	var y float64
	x = math.Abs(x)
	if x < 1.0 {
		y = ((12-9*b-6*c)*x*x*x + (-18+12*b+6*c)*x*x + (6 - 2*b)) / 6
	} else if x < 2.0 {
		y = ((-b-6*c)*x*x*x + (6*b+30*c)*x*x + (-12*b-48*c)*x + (8*b + 24*c)) / 6
	}
	return y
}

func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	return math.Sin(math.Pi*x) / (math.Pi * x)
}

func init() {
	NearestNeighbor = ResampleFilter{
		Support: 0.0, // special case - not applying the filter
	}

	Box = ResampleFilter{
		Support: 0.5,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x <= 0.5 {
				return 1.0
			}
			return 0
		},
	}

	Linear = ResampleFilter{
		Support: 1.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 1.0 {
				return 1.0 - x
			}
			return 0
		},
	}

	Hermite = ResampleFilter{
		Support: 1.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 1.0 {
				return bcspline(x, 0.0, 0.0)
			}
			return 0
		},
	}

	MitchellNetravali = ResampleFilter{
		Support: 2.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 2.0 {
				return bcspline(x, 1.0/3.0, 1.0/3.0)
			}
			return 0
		},
	}

	CatmullRom = ResampleFilter{
		Support: 2.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 2.0 {
				return bcspline(x, 0.0, 0.5)
			}
			return 0
		},
	}

	BSpline = ResampleFilter{
		Support: 2.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 2.0 {
				return bcspline(x, 1.0, 0.0)
			}
			return 0
		},
	}

	Gaussian = ResampleFilter{
		Support: 2.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 2.0 {
				return math.Exp(-2 * x * x)
			}
			return 0
		},
	}

	Bartlett = ResampleFilter{
		Support: 3.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 3.0 {
				return sinc(x) * (3.0 - x) / 3.0
			}
			return 0
		},
	}

	Lanczos = ResampleFilter{
		Support: 3.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 3.0 {
				return sinc(x) * sinc(x/3.0)
			}
			return 0
		},
	}

	Hann = ResampleFilter{
		Support: 3.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 3.0 {
				return sinc(x) * (0.5 + 0.5*math.Cos(math.Pi*x/3.0))
			}
			return 0
		},
	}

	Hamming = ResampleFilter{
		Support: 3.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 3.0 {
				return sinc(x) * (0.54 + 0.46*math.Cos(math.Pi*x/3.0))
			}
			return 0
		},
	}

	Blackman = ResampleFilter{
		Support: 3.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 3.0 {
				return sinc(x) * (0.42 - 0.5*math.Cos(math.Pi*x/3.0+math.Pi) + 0.08*math.Cos(2.0*math.Pi*x/3.0))
			}
			return 0
		},
	}

	Welch = ResampleFilter{
		Support: 3.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 3.0 {
				return sinc(x) * (1.0 - (x * x / 9.0))
			}
			return 0
		},
	}

	Cosine = ResampleFilter{
		Support: 3.0,
		Kernel: func(x float64) float64 {
			x = math.Abs(x)
			if x < 3.0 {
				return sinc(x) * math.Cos((math.Pi/2.0)*(x/3.0))
			}
			return 0
		},
	}
}

