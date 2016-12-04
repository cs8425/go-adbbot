package adbbot

import (
	"image"
//	"image/color"
	"sync"
	"time"
)


type Match struct {
	Pt image.Point
	Val float64
	ValInt int64
}

func abs(a, b uint8) (int64){
	c := int64(a) - int64(b)
	return c * c
}

func (b Bot) FindExistReg(tmpl *Tmpl, times int, delay int) (x int, y int, val float64){

	for i := 0; i < times; i++ {
		Vlogln(4, "Screencap()", i)
		img, err := b.Screencap()
		if err != nil {
			continue
		}

		if !tmpl.Region.Empty() {
			Vlogln(4, "crop", i)
			reg := img.Bounds().Intersect(tmpl.Region)
			img = img.(*image.NRGBA).SubImage(reg)
		}

		Vlogln(4, "FindP()", i)
		timeStart()
		if b.TargetScreen != nil {
			scriptsize := b.TargetScreen.Size()
			screensize := b.Screen.Size()
			tmplsize := tmpl.Image.Bounds().Size()
			newX := tmplsize.X * screensize.X / scriptsize.X
			newY := tmplsize.Y * screensize.Y / scriptsize.Y
			Vlogln(5, "Resize to", newX, newY)
			dstImage := Resize(tmpl.Image, newX, newY, Lanczos)
			timeEnd("Resize()")
			x, y, val = FindP(img, dstImage)
		} else {
			x, y, val = FindP(img, tmpl.Image)
		}
		timeEnd("FindP()")
		if x != -1 && y != -1 {
			Vlogln(3, "FindExistP()", x, y, val)
			return
		}
		time.Sleep(time.Millisecond * time.Duration(delay))
	}

	Vlogln(3, "FindExistP()", x, y, val)
	return
}

func (b Bot) FindExistP(subimg image.Image, times int, delay int) (x int, y int, val float64){

	for i := 0; i < times; i++ {
		Vlogln(4, "Screencap()", i)
		img, err := b.Screencap()
		if err != nil {
			continue
		}

		Vlogln(4, "FindP()", i)
		timeStart()
		x, y, val = FindP(img, subimg)
		timeEnd("FindP()")
		if x != -1 && y != -1 {
			Vlogln(3, "FindExistP()", x, y, val)
			return
		}
		time.Sleep(time.Millisecond * time.Duration(delay))
	}

	Vlogln(3, "FindExistP()", x, y, val)
	return
}

func FindP(img image.Image, subimg image.Image) (x int, y int, val float64) {

	x = -1
	y = -1

	startX := img.Bounds().Min.X
	endX := img.Bounds().Max.X - subimg.Bounds().Dx()

	startY := img.Bounds().Min.Y
	endY := img.Bounds().Max.Y - subimg.Bounds().Dy()

	var min int64 = int64(subimg.Bounds().Dx() * subimg.Bounds().Dy() * 255 * 255 * 3) / 32
//	var min int64 = 0x7fffffffffffffff

	Vlogln(4, "Find @ = ", startX, endX, startY, endY)

	if nrgba, ok := img.(*image.NRGBA); ok {
		if snrgba, ok := subimg.(*image.NRGBA); ok {

			var mutex = &sync.Mutex{}

			parallel(endY - startY, 1, func(partStart, partEnd int) {
//				Vlogln(2, "partStart, partEnd = ", partStart, partEnd)
				partStart += startY
				partEnd += startY
				for i := partStart; i < partEnd; i++ {
					for j := startX; j < endX; j++ {

						tmp := CmpAt(nrgba, snrgba, j, i, min)
						mutex.Lock()
						if tmp < min {
							min = tmp
							x = j
							y = i
						}
						mutex.Unlock()
					}
				}
//				Vlogln(2, "min, x, y = ", min, x, y)
			})

		}
	}

	val = (1 - (float64(min) / float64(255 * 255 * 3 * subimg.Bounds().Dy() * subimg.Bounds().Dx())))
	if x == -1 && y == -1 {
		return -1, -1, 0
	} else {
		return x, y, val
	}
}

func Find(img image.Image, subimg image.Image) (x int, y int, val float64) {

	x = -1
	y = -1

	startX := img.Bounds().Min.X
	endX := img.Bounds().Max.X - subimg.Bounds().Dx()

	startY := img.Bounds().Min.Y
	endY := img.Bounds().Max.Y - subimg.Bounds().Dy()

	var min int64 = int64(subimg.Bounds().Dx() * subimg.Bounds().Dy() * 255 * 255 * 3) / 32
//	var min int64 = 0x7fffffffffffffff

	Vlogln(4, "Find @ = ", startX, endX, startY, endY)

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
	if x == -1 && y == -1 {
		return -1, -1, 0
	} else {
		return x, y, val
	}
}

func CmpAt(img *image.NRGBA, subimg *image.NRGBA, offX int, offY int, limit int64) (int64) {

	if limit == 0 {
		return 0
	}

	var diff int64 = 0

	dX := subimg.Bounds().Dx()
	if offX + dX > img.Bounds().Max.X {
		dX = img.Bounds().Max.X
		return 0
	}

	dY := subimg.Bounds().Dy()
	if offY + dY > img.Bounds().Max.Y {
		dY = img.Bounds().Max.Y
		return 0
	}

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


