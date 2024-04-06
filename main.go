package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strconv"
	"strings"
	"syscall/js"
)

type ResizeType int

const (
	Vertical ResizeType = iota
	Horizontal
)

type JPGImg struct {
	FileName string
	Img      []byte
}

// args[0]: resizeType, args[1]: specifiedfileSize, args[2]:quality, args[3]: fileCount, args[4...]: files
func Convert(this js.Value, args []js.Value) interface{} {
	offset := 4
	var tmp string = args[0].String()
	resizeType, err := strconv.Atoi(tmp)
	if err != nil {
		fmt.Println(err)
		printAlert("リサイズ対象の判定に失敗しました")
		return nil
	}

	tmp = args[1].String()
	specifiedfileSize, err := strconv.Atoi(tmp)
	if err != nil {
		printAlert("添付されたファイルカウントの取得に失敗しました")
		return nil
	}

	tmp = args[2].String()
	quality, err := strconv.Atoi(tmp)
	if err != nil {
		printAlert("指定された品質の取得に失敗しました")
		return nil
	}

	tmp = args[3].String()
	fileCount, err := strconv.Atoi(tmp)
	if err != nil {
		printAlert("添付されたファイルカウントの取得に失敗しました")
		return nil
	}

	var JPGImgs []JPGImg

	for i := offset; i < offset+fileCount; i++ {
		base64Decode, err := base64.StdEncoding.DecodeString(args[i].Get("base64").String())
		if err != nil {
			printAlert("添付されたPNGデータの取得に失敗しました")
			return nil
		}

		img, err := png.Decode(strings.NewReader(string(base64Decode)))
		if err != nil {
			printAlert("PNGデータのデコードに失敗しました")
			return nil
		}

		if specifiedfileSize >= 50 {
			img = resizeImage(ResizeType(resizeType), specifiedfileSize, img)
		}

		imgWithWhite := fillTransparentWhite(img)

		var b bytes.Buffer
		if err := jpeg.Encode(bufio.NewWriter(&b), imgWithWhite, &jpeg.Options{Quality: quality}); err != nil {
			printAlert("JPGデータへのエンコードに失敗しました")
			return nil
		}

		var fileName string = args[i].Get("fileName").String()
		ext := filepath.Ext(fileName)
		JPGImgs = append(JPGImgs, JPGImg{FileName: strings.TrimSuffix(fileName, ext), Img: b.Bytes()})
	}

	zipData, err := createZip(&JPGImgs)
	if err != nil {
		printAlert("zipファイル作成中にエラーが発生しました")
		return nil
	}

	attachData(zipData, "archive", ".zip")
	return nil
}

// PNG画像のアルファチャネルを白く塗りつぶす処理
func fillTransparentWhite(img image.Image) image.Image {
	bounds := img.Bounds()
	newImg := image.NewRGBA(bounds)

	white := color.RGBA{255, 255, 255, 255}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha == 0 {
				newImg.Set(x, y, white)
			} else {
				newImg.Set(x, y, img.At(x, y))
			}
		}
	}
	return newImg
}

func resizeImage(resizeType ResizeType, specifiedfileSize int, img image.Image) image.Image {
	origBounds := img.Bounds()
	origWidth := origBounds.Dx()
	origHeight := origBounds.Dy()
	var ratio float64

	if resizeType == Horizontal {
		ratio = float64(specifiedfileSize) / float64(origWidth)
	} else {
		ratio = float64(specifiedfileSize) / float64(origHeight)
	}
	//fmt.Printf("orig width: %d, height: %d, rate: %f\n", origWidth, origHeight, ratio)

	newWidth := int(float64(origWidth) * ratio)
	newHeight := int(float64(origHeight) * ratio)
	//fmt.Printf("new width: %d, height: %d, rate: %f\n", newWidth, newHeight, ratio)

	resizeImg := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			origX := int(float64(x) / ratio)
			origY := int(float64(y) / ratio)
			resizeImg.Set(x, y, img.At(origX, origY))
		}
	}

	return resizeImg
}

func createZip(data *[]JPGImg) ([]byte, error) {
	var zipData bytes.Buffer
	zipWriter := zip.NewWriter(&zipData)

	for _, content := range *data {
		fileName := fmt.Sprintf("%s.jpg", content.FileName)
		fileWriter, err := zipWriter.Create(fileName)
		if err != nil {
			return nil, err
		}

		_, err = fileWriter.Write(content.Img)
		if err != nil {
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return zipData.Bytes(), nil
}

func attachData(data []byte, fileName string, ext string) {
	document := js.Global().Get("document")
	el := document.Call("getElementById", "output-file")
	encode := base64.StdEncoding.EncodeToString(data)
	dataUri := fmt.Sprintf("data:%s;base64,%s", "application/zip", encode)
	el.Set("href", dataUri)
	el.Set("download", fileName+ext)
	el.Call("click")
}

func printAlert(msg string) {
	document := js.Global().Get("document")
	el := document.Call("getElementById", "err-msg-spn")
	el.Set("innerText", msg)
}

func main() {
	ch := make(chan struct{}, 0)
	js.Global().Set("Convert", js.FuncOf(Convert))
	<-ch
}
