package adbbot

import (
	"image"
	"math"
	"sync/atomic"
	"time"
)

func abs(a, b uint8) int64 {
	// c := int64(a) - int64(b)
	// y := c >> 63
	// return (c ^ y) - y

	c := int(a) - int(b)
	return int64(c * c)
}

func FindExistReg(b Bot, tmpl *Tmpl, times int, delay int) (x int, y int, val float64) {

	for i := 0; i < times; i++ {
		Vln(5, "Screencap()", i)
		img, err := b.Screencap()
		if err != nil {
			continue
		}

		Vln(5, "FindP()", i)
		// timeStart()
		x, y, val = FindInTmpl(img, tmpl)
		// timeEnd("FindP()")
		if x != -1 && y != -1 {
			Vln(4, "FindExistP()", x, y, val)
			return
		}
		time.Sleep(time.Millisecond * time.Duration(delay))
	}

	Vln(4, "FindExistP()", x, y, val)
	return
}

func FindCachedReg(b Bot, tmpl *Tmpl, delay int) (x int, y int, val float64) {
	img := b.GetLastScreencap()
	x, y, val = FindInTmpl(img, tmpl)
	Vln(4, "FindCachedReg()", x, y, val)
	time.Sleep(time.Millisecond * time.Duration(delay))
	return
}

func FindInTmpl(img image.Image, tmpl *Tmpl) (x int, y int, val float64) {
	if !tmpl.Region.Empty() {
		Vln(5, "crop", tmpl)
		var reg image.Rectangle
		reg = img.Bounds().Intersect(tmpl.Region)
		img = img.(*image.NRGBA).SubImage(reg)
	}

	Vln(5, "FindInTmpl()", tmpl)
	// timeStart()
	x, y, val = FindP(img, tmpl.Image)
	// timeEnd("FindInTmpl()")
	Vln(4, "FindInTmpl()", x, y, val)
	return
}

func FindExistImg(b Bot, subimg image.Image, times int, delay int) (x int, y int, val float64) {

	for i := 0; i < times; i++ {
		Vln(5, "Screencap()", i)
		img, err := b.Screencap()
		if err != nil {
			continue
		}

		Vln(5, "FindP()", i)
		// timeStart()
		x, y, val = FindP(img, subimg)
		// timeEnd("FindP()")
		if x != -1 && y != -1 {
			Vln(4, "FindExistP()", x, y, val)
			return
		}
		time.Sleep(time.Millisecond * time.Duration(delay))
	}

	Vln(4, "FindExistP()", x, y, val)
	return
}

func FindP(img image.Image, subimg image.Image) (x int, y int, val float64) {
	timeStart()

	x = -1
	y = -1
	val = 0

	startX := img.Bounds().Min.X
	endX := img.Bounds().Max.X - subimg.Bounds().Dx()

	startY := img.Bounds().Min.Y
	endY := img.Bounds().Max.Y - subimg.Bounds().Dy()

	var min int64 = int64(subimg.Bounds().Dx()*subimg.Bounds().Dy()*255*255*3) / 32
	// var min int64 = 0x7fffffffffffffff

	Vln(5, "Find @ = ", startX, endX, startY, endY)

	if nrgba, ok := img.(*image.NRGBA); ok {
		if snrgba, ok := subimg.(*image.NRGBA); ok {

			type cmpRet struct {
				X int
				Y int
				V int64
			}

			reduceCh := make(chan cmpRet, 8)
			go func() {
				for {
					ret, ok := <-reduceCh
					if !ok {
						return
					}

					if ret.V < min {
						x = ret.X
						y = ret.Y
						atomic.StoreInt64(&min, ret.V)
					}
				}
			}()

			wg := parallelSpawn(startY, endY, func(ys <-chan int) {
				for y := range ys {
					for j := startX; j < endX; j++ {
						localMin := atomic.LoadInt64(&min)
						tmp := CmpAt(nrgba, snrgba, j, y, localMin)
						if tmp < localMin {
							reduceCh <- cmpRet{
								X: j,
								Y: y,
								V: tmp,
							}
						}
					}
				}
				// Vln(2, "min, x, y = ", min, x, y)
			})
			wg.Wait()
			close(reduceCh)
		}
	}
	val = calcVal(subimg, min)

	timeEnd("FindP()")

	return x, y, val
}

func Find(img image.Image, subimg image.Image) (x int, y int, val float64) {
	timeStart()

	x = -1
	y = -1
	val = 0

	startX := img.Bounds().Min.X
	endX := img.Bounds().Max.X - subimg.Bounds().Dx()

	startY := img.Bounds().Min.Y
	endY := img.Bounds().Max.Y - subimg.Bounds().Dy()

	var min int64 = int64(subimg.Bounds().Dx()*subimg.Bounds().Dy()*255*255*3) / 32
	// var min int64 = 0x7fffffffffffffff

	Vln(4, "Find @ = ", startX, endX, startY, endY)

	if nrgba, ok := img.(*image.NRGBA); ok {
		if snrgba, ok := subimg.(*image.NRGBA); ok {

			for i := startY; i < endY; i++ {
				for j := startX; j < endX; j++ {

					tmp := CmpAt(nrgba, snrgba, j, i, min)
					if tmp < min {
						min = tmp
						x = j
						y = i
					}

				}
			}

		}
	}
	val = calcVal(subimg, min)

	timeEnd("Find()")

	return x, y, val
}

func CmpAt(img *image.NRGBA, subimg *image.NRGBA, offX int, offY int, limit int64) int64 {

	if limit <= 0 {
		return 0
	}

	var diff int64 = 0

	dX := subimg.Bounds().Dx()
	if offX+dX > img.Bounds().Max.X {
		dX = img.Bounds().Max.X
		return limit
	}

	dY := subimg.Bounds().Dy()
	if offY+dY > img.Bounds().Max.Y {
		dY = img.Bounds().Max.Y
		return limit
	}

	// BCE
	bound1 := img.PixOffset(offX, offY)
	bound2 := img.PixOffset(dX+offX, dY+offY) + 3
	_ = img.Pix[bound1:bound2]

	bound1 = subimg.PixOffset(0, 0)
	bound2 = subimg.PixOffset(dX-1, dY-1) + 3
	_ = subimg.Pix[bound1:bound2]

	// start
	for i := 0; i < dY; i++ {
		for j := 0; j < dX; j++ {

			oi := img.PixOffset(j+offX, i+offY)
			si := subimg.PixOffset(j, i)

			if subimg.Pix[si+3] == 0 { // alpha = 0 >> transparent, skip calculate
				continue
			}

			diff += abs(img.Pix[oi+0], subimg.Pix[si+0])
			diff += abs(img.Pix[oi+1], subimg.Pix[si+1])
			diff += abs(img.Pix[oi+2], subimg.Pix[si+2])
			// diff += abs(img.Pix[oi + 3], subimg.Pix[si + 3])

			if diff > limit {
				goto END
			}
		}
	}

END:
	return diff
}

func calcVal(img image.Image, diff int64) float64 {
	var alpha int64 = 0

	subimg, ok := img.(*image.NRGBA)
	if !ok {
		return 0.0
	}

	startX := subimg.Bounds().Min.X
	endX := subimg.Bounds().Max.X

	startY := subimg.Bounds().Min.Y
	endY := subimg.Bounds().Max.Y

	bound1 := subimg.PixOffset(startX, startY)
	bound2 := subimg.PixOffset(endX-1, endY-1) + 3
	_ = subimg.Pix[bound1:bound2]

	for i := startX; i < endY; i++ {
		for j := startX; j < endX; j++ {
			si := subimg.PixOffset(j, i)
			// alpha = 0 >> transparent, skip calculate
			// alpha += int64(subimg.Pix[si + 3])
			a := int64(subimg.Pix[si+3])
			alpha += a * a
		}
	}

	if alpha == 0 {
		return 1
	}

	// val := float64(diff) / float64(alpha * 3)
	// val = 1 - val
	val := float64(diff) / float64(alpha*3)
	val = 1 - math.Sqrt(val)

	Vln(4, "calcVal()", diff, val, alpha, subimg.Bounds().Dx()*subimg.Bounds().Dy())

	return val
}
