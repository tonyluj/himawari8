package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
)

var (
	bucketName        string
	now               bool
	serviceAccountKey string
	client            *storage.Client
)

type Resolution struct {
	Width  int
	Height int
	Name   string
}

func Merge(src [][]io.Reader) (img *image.RGBA, err error) {
	rt := image.Rectangle{image.Point{0, 0}, image.Point{550 * 20, 550 * 20}}
	img = image.NewRGBA(rt)

	for x := 0; x < 20; x++ {
		for y := 0; y < 20; y++ {
			i, er := png.Decode(src[x][y])
			if er != nil {
				err = errors.Wrap(er, "merge error")
				return
			}

			r := image.Rectangle{image.Point{550 * x, 550 * y}, image.Point{550 * (x + 1), 550 * (y + 1)}}
			draw.Draw(img, r, i, image.Point{0, 0}, draw.Src)
		}
	}

	return
}

func Area(x, y int, t time.Time, w io.Writer) (err error) {
	ft := t.Format("2006/01/02/150400")
	resp, err := http.Get(fmt.Sprintf("http://himawari8-dl.nict.go.jp/himawari8/img/D531106/20d/550/%v_%v_%v.png", ft, x, y))
	if err != nil {
		err = errors.Wrap(err, "area error")
		return
	}
	defer resp.Body.Close()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		err = errors.Wrap(err, "area error")
		return
	}
	return
}

func Draw(img image.Image, r Resolution) (result *image.RGBA) {
	backgroudColor := img.At(0, 0)

	// draw a canvas
	rt := image.Rectangle{image.Point{0, 0}, image.Point{r.Width, r.Height}}
	result = image.NewRGBA(rt)
	for x := result.Rect.Min.X; x < result.Rect.Max.X; x++ {
		for y := result.Rect.Min.Y; y < result.Rect.Max.Y; y++ {
			result.Set(x, y, backgroudColor)
		}
	}

	// resize origin
	var (
		resizeWidth  int
		resizeHeight int
	)
	if r.Height < r.Width {
		resizeWidth = r.Height * 8 / 10
	} else {
		resizeWidth = r.Width * 8 / 10
	}
	resizeHeight = resizeWidth

	resizeImg := resize.Resize(uint(resizeWidth), uint(resizeHeight), img, resize.Lanczos3)

	var (
		startX int = (r.Width - resizeWidth) / 2
		startY int = (r.Height - resizeHeight) / 2
		endX   int = resizeWidth + startX
		endY   int = resizeHeight + startY
	)

	// draw main
	m := image.Rectangle{image.Point{startX, startY}, image.Point{endX, endY}}
	draw.Draw(result, m, resizeImg, image.Point{0, 0}, draw.Src)
	return
}

func Generate(name string, t time.Time, resolutions ...Resolution) (err error) {
	files := make([][]*os.File, 0, 20)

	for x := 0; x < 20; x++ {
		fs := make([]*os.File, 0, 20)

		for y := 0; y < 20; y++ {
			fileName := fmt.Sprintf("%v_%d_%d.png", t.Format("2006-01-02_15:04:05"), x, y)

			f, er := ioutil.TempFile("", fileName)
			if er != nil {
				err = errors.Wrap(er, "generate error")
				return
			}

			err = Area(x, y, t, f)
			if err != nil {
				err = errors.Wrap(err, "generate error")
				return
			}

			f.Seek(0, os.SEEK_SET)
			fs = append(fs, f)
		}

		files = append(files, fs)
	}

	src := make([][]io.Reader, 20)
	for x := 0; x < 20; x++ {
		inner := make([]io.Reader, 20)
		src[x] = inner

		for y := 0; y < 20; y++ {
			src[x][y] = files[x][y]
		}
	}

	img, err := Merge(src)
	if err != nil {
		err = errors.Wrap(err, "generate error")
		return
	}

	for _, r := range resolutions {
		rimg := Draw(img, r)

		w := CloudFile(fmt.Sprintf("%v_%v.jpg", name, r.Name))
		err = jpeg.Encode(w, rimg, nil)
		if err != nil {
			err = errors.Wrap(err, "generate error")
			return
		}
		w.Close()
	}

	for x := 0; x < 20; x++ {
		for y := 0; y < 20; y++ {
			files[x][y].Close()

			os.Remove(files[x][y].Name())
		}
	}
	return
}

func CloudFile(name string) (w io.WriteCloser) {
	bucket := client.Bucket(bucketName)
	obj := bucket.Object(name)
	w = obj.NewWriter(context.Background())
	return
}

func init() {
	flag.BoolVar(&now, "now", false, "generate now")
	flag.StringVar(&serviceAccountKey, "key", "", "Google cloud service account key")
	flag.StringVar(&bucketName, "bucket", "", "Google cloud storage bucket")
	flag.Parse()

	var err error
	client, err = storage.NewClient(context.Background(), option.WithServiceAccountFile(serviceAccountKey))
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	rs := make([]Resolution, 0, 5)
	//rs = append(rs, Resolution{7680, 4320, "8k"})
	//rs = append(rs, Resolution{5120, 2160, "5k"})
	rs = append(rs, Resolution{3840, 2160, "4k"})
	rs = append(rs, Resolution{2880, 1800, "3k_16:10"})
	//rs = append(rs, Resolution{1920, 1080, "1k"})

	if now {
		t := time.Now()
		log.Println("Generate Begin")

		m := t.Minute()/10*10 - 30 - t.Minute()
		nt := t.Add(time.Duration(m) * time.Minute)
		fileName := fmt.Sprintf("earth_%v", t.Format("200601021504"))

		err := Generate(fileName, nt.In(time.UTC), rs...)
		if err != nil {
			log.Println(err)
			return
		}

		log.Println("Generate Done")
		return
	}

	ticker := time.NewTicker(time.Minute * 10)
	defer ticker.Stop()

	log.Println("Start")

	for {
		select {
		case t := <-ticker.C:
			log.Println("Generate Begin")

			m := t.Minute()/10*10 - 30 - t.Minute()
			nt := t.Add(time.Duration(m) * time.Minute)
			fileName := fmt.Sprintf("earth_%v", t.Format("200601021504"))

			err := Generate(fileName, nt.In(time.UTC), rs...)
			if err != nil {
				log.Println(err)
				continue
			}

			log.Println("Generate Done")
		}
	}
}
