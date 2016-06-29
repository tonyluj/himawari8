package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func Merge(src [][]io.Reader, w io.Writer) (err error) {
	rt := image.Rectangle{image.Point{0, 0}, image.Point{550 * 20, 550 * 20}}
	img := image.NewRGBA(rt)

	for x := 0; x < 20; x++ {
		for y := 0; y < 20; y++ {
			i, err := png.Decode(src[x][y])
			if err != nil {
				return err
			}

			r := image.Rectangle{image.Point{550 * x, 550 * y}, image.Point{550 * (x + 1), 550 * (y + 1)}}
			draw.Draw(img, r, i, image.Point{0, 0}, draw.Src)
		}
	}

	err = png.Encode(w, img)
	return
}

func Area(x, y int, t time.Time, w io.Writer) (err error) {
	ft := t.Format("2006/01/02/150400")
	resp, err := http.Get(fmt.Sprintf("http://himawari8-dl.nict.go.jp/himawari8/img/D531106/20d/550/%v_%v_%v.png", ft, x, y))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return
	}
	return
}

func Generate(name string, t time.Time) (err error) {
	src := make([][]io.Reader, 0, 20)
	for x := 0; x < 20; x++ {
		inner := make([]io.Reader, 0, 20)

		for y := 0; y < 20; y++ {
			b := new(bytes.Buffer)
			err = Area(x, y, t, b)
			if err != nil {
				return
			}

			inner = append(inner, b)
		}

		src = append(src, inner)
	}

	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return
	}
	defer f.Close()

	err = Merge(src, f)
	if err != nil {
		return
	}

	return
}

func main() {
	ticker := time.NewTicker(time.Minute * 10)
	defer ticker.Stop()

	log.Println("Start")

	for {
		select {
		case t := <-ticker.C:
			log.Println("Generate Begin")

			m := t.Minute()/10*10 - 30 - t.Minute()
			nt := t.Add(time.Duration(m) * time.Minute)
			err := Generate(fmt.Sprintf("result_%v.png", t.Format("200601021504")), nt.In(time.UTC))
			if err != nil {
				log.Println(err)
				continue
			}

			log.Println("Generate Done")
		}
	}
}
