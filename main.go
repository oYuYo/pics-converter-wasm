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

type JPGImg struct {
	FileName string
	Img      []byte
}

// args[0]: specifiedfileSize, args[1]: fileCount, args[2]: files
func Convert(this js.Value, args []js.Value) interface{} {

	var tmp string = args[0].String()
	_, err := strconv.Atoi(tmp)
	if err != nil {
		fmt.Println(err)
		printAlert("指定された縮小サイズが不正です")
		return nil
	}

	tmp = args[1].String()
	fileCount, err := strconv.Atoi(tmp)
	if err != nil {
		printAlert("添付されたファイルカウントの取得に失敗しました")
		return nil
	}

	var JPGImgs []JPGImg

	for i := 2; i < 2+fileCount; i++ {
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

		imgWithWhite := fillTransparentWhite(img)

		var b bytes.Buffer
		if err := jpeg.Encode(bufio.NewWriter(&b), imgWithWhite, nil); err != nil {
			printAlert("JPGデータへのエンコードに失敗しました")
			return nil
		}

		var fileName string = args[i].Get("fileName").String()
		ext := filepath.Ext(fileName)
		JPGImgs = append(JPGImgs, JPGImg{FileName: strings.TrimSuffix(fileName, ext), Img: b.Bytes()})
	}

	b, err := createZip(&JPGImgs)
	if err != nil {
		printAlert("zipファイル作成中にエラーが発生しました")
		return nil
	}

	attachData(b, "archive", ".zip")
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

func SizeReduction() error {

	return nil
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
