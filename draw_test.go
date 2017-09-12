package main

import (
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

func TestDraw(t *testing.T) {
	f, err := os.Open("src.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	rimg := Draw(img, Resolution{3840, 2160, "1k"})

	fn, err := os.OpenFile("dst.jpg", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer fn.Close()

	err = jpeg.Encode(fn, rimg, nil)
	if err != nil {
		t.Fatal(err)
	}
}
