package adbbot

import (
	"image"
	"sync/atomic"
	"time"
)

func abs(a, b uint8) (int64){
	c := int64(a) - int64(b)
	return c * c
}

func FindExistReg(b Bot, tmpl *Tmpl, times int, delay int) (x int, y int, val float64){

	for i := 0; i < times; i++ {
		Vln(5, "Screencap()", i)
		img, err := b.Screencap()
		if err != nil {
			continue
		}

		if !tmpl.Region.Empty() {
			Vln(5, "crop", i)
			var reg image.Rectangle
			/*if b.TargetScreen != nil {
				scriptsize := b.TargetScreen.Size()
				screensize := b.ScreenBounds.Size()
				reg = tmpl.Region
				newMinX := reg.Min.X * screensize.X / scriptsize.X
				newMaxX := reg.Max.X * screensize.X / scriptsize.X
				newMinY := reg.Min.Y * screensize.Y / scriptsize.Y
				newMaxY := reg.Max.Y * screensize.Y / scriptsize.Y
				reg = image.Rect(newMinX, newMinY, newMaxX, newMaxY)
				Vln(6, "crop Resize to", reg)
			} else {
				reg = img.Bounds().Intersect(tmpl.Region)
			}*/
			reg = img.Bounds().Intersect(tmpl.Region)
			img = img.(*image.NRGBA).SubImage(reg)
		}

		Vln(5, "FindP()", i)
//		timeStart()
		/*if b.TargetScreen != nil {
			scriptsize := b.TargetScreen.Size()
			screensize := b.ScreenBounds.Size()
			tmplsize := tmpl.Image.Bounds().Size()
			newX := tmplsize.X * screensize.X / scriptsize.X
			newY := tmplsize.Y * screensize.Y / scriptsize.Y
			if (screensize.X == scriptsize.X) && (screensize.Y == scriptsize.Y) {
				x, y, val = FindP(img, tmpl.Image)
			} else {
				Vln(5, "Resize to", newX, newY)
				dstImage := Resize(tmpl.Image, newX, 0, Lanczos)
				timeEnd("Resize()")
				x, y, val = FindP(img, dstImage)
			}
		} else {
			x, y, val = FindP(img, tmpl.Image)
		}*/
		x, y, val = FindP(img, tmpl.Image)
//		timeEnd("FindP()")
		if x != -1 && y != -1 {
			Vln(4, "FindExistP()", x, y, val)
			return
		}
		time.Sleep(time.Millisecond * time.Duration(delay))
	}

	Vln(4, "FindExistP()", x, y, val)
	return
}

func FindRegCached(b Bot, tmpl *Tmpl, delay int) (x int, y int, val float64){
	img := b.GetLastScreencap()

	if !tmpl.Region.Empty() {
		Vln(5, "crop", tmpl)
		var reg image.Rectangle
		reg = img.Bounds().Intersect(tmpl.Region)
		img = img.(*image.NRGBA).SubImage(reg)
	}

	Vln(5, "FindP()", tmpl)
//	timeStart()
	x, y, val = FindP(img, tmpl.Image)
//	timeEnd("FindP()")
	if x != -1 && y != -1 {
		Vln(4, "FindExistP()", x, y, val)
		return
	}

	Vln(4, "FindExistP()", x, y, val)
	time.Sleep(time.Millisecond * time.Duration(delay))
	return
}

func FindExistP(b Bot, subimg image.Image, times int, delay int) (x int, y int, val float64){

	for i := 0; i < times; i++ {
		Vln(5, "Screencap()", i)
		img, err := b.Screencap()
		if err != nil {
			continue
		}

		Vln(5, "FindP()", i)
//		timeStart()
		x, y, val = FindP(img, subimg)
//		timeEnd("FindP()")
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

	startX := img.Bounds().Min.X
	endX := img.Bounds().Max.X - subimg.Bounds().Dx()

	startY := img.Bounds().Min.Y
	endY := img.Bounds().Max.Y - subimg.Bounds().Dy()

	var min int64 = int64(subimg.Bounds().Dx() * subimg.Bounds().Dy() * 255 * 255 * 3) / 32
//	var min int64 = 0x7fffffffffffffff

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
					ret, ok := <- reduceCh
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
							reduceCh <- cmpRet {
								X: j,
								Y: y,
								V: tmp,
							}
						}
					}
				}
//				Vln(2, "min, x, y = ", min, x, y)
			})
			wg.Wait()
			close(reduceCh)
		}
	}
	val = (1 - (float64(min) / float64(255 * 255 * 3 * subimg.Bounds().Dy() * subimg.Bounds().Dx())))

	timeEnd("FindP2()")

	if x == -1 && y == -1 {
		return -1, -1, 0
	} else {
		return x, y, val
	}
}

func Find(img image.Image, subimg image.Image) (x int, y int, val float64) {
	timeStart()

	x = -1
	y = -1

	startX := img.Bounds().Min.X
	endX := img.Bounds().Max.X - subimg.Bounds().Dx()

	startY := img.Bounds().Min.Y
	endY := img.Bounds().Max.Y - subimg.Bounds().Dy()

	var min int64 = int64(subimg.Bounds().Dx() * subimg.Bounds().Dy() * 255 * 255 * 3) / 32
//	var min int64 = 0x7fffffffffffffff

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

	val = (1 - (float64(min) / float64(255 * 255 * 3 * subimg.Bounds().Dy() * subimg.Bounds().Dx())))

	timeEnd("Find()")

	if x == -1 && y == -1 {
		return -1, -1, 0
	} else {
		return x, y, val
	}
}

func CmpAt(img *image.NRGBA, subimg *image.NRGBA, offX int, offY int, limit int64) (int64) {

	if limit <= 0 {
		return 0
	}

	var diff int64 = 0

	dX := subimg.Bounds().Dx()
	if offX + dX > img.Bounds().Max.X {
		dX = img.Bounds().Max.X
		return limit
	}

	dY := subimg.Bounds().Dy()
	if offY + dY > img.Bounds().Max.Y {
		dY = img.Bounds().Max.Y
		return limit
	}

	// BCE
	bound1 := img.PixOffset(offX, offY)
	bound2 := img.PixOffset(dX + offX, dY + offY)
	_ = img.Pix[bound1:bound2]

	bound1 = subimg.PixOffset(0, 0)
	bound2 = subimg.PixOffset(dX-1, dY-1)
	_ = subimg.Pix[bound1:bound2]

	// start
	for i := 0; i < dY; i++ {
		for j := 0; j < dX; j++ {

			oi := img.PixOffset(j + offX, i + offY)
			si := subimg.PixOffset(j, i)

			diff += abs(img.Pix[oi + 0], subimg.Pix[si + 0])
			diff += abs(img.Pix[oi + 1], subimg.Pix[si + 1])
			diff += abs(img.Pix[oi + 2], subimg.Pix[si + 2])
			//diff += abs(img.Pix[oi + 3], subimg.Pix[si + 3])

			if diff > limit {
				goto END
			}
		}
	}


END:
	return diff
}


