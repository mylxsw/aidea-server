package image

import (
	"bytes"
	"fmt"
	"github.com/fogleman/gg"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/skip2/go-qrcode"
	"strings"
)

type Imager struct {
	fontPath string
}

func New(fontPath string) *Imager {
	return &Imager{fontPath: fontPath}
}

func (builder *Imager) TextImage(text string, size int) ([]byte, error) {
	dc := gg.NewContext(size, size)
	dc.SetRGB(0, 0, 0)

	dc.Clear()
	dc.SetRGB(1, 1, 1)

	var points float64
	textSize := misc.WordCount(strings.ReplaceAll(text, "\n", ""))
	switch textSize {
	case 1:
		points = 600
	case 2:
		points = 450
	case 3:
		points = 300
	case 4:
		points = 200
	case 5:
		points = 150
	case 6:
		points = 120
	default:
		points = 80
	}

	if builder.fontPath != "" {
		if err := dc.LoadFontFace(builder.fontPath, points); err != nil {
			return nil, fmt.Errorf("加载字体文件失败: %w", err)
		}
	}

	//sWidth, sHeight := dc.MeasureString(text)

	//dc.DrawString(text, (float64(size)-sWidth)/2, (float64(size)+sHeight)/2)
	dc.DrawStringWrapped(text, float64(size/2), float64(size/2), 0.5, 0.5, float64(size), 1.2, gg.AlignCenter)

	buf := bytes.NewBuffer(nil)
	if err := dc.EncodePNG(buf); err != nil {
		return nil, fmt.Errorf("编码 PNG 数据失败: %w", err)
	}

	return buf.Bytes(), nil
}

func (builder *Imager) QR(link string, size int) ([]byte, error) {
	qr, err := qrcode.New(link, qrcode.Medium)
	if err != nil {
		return nil, err
	}

	qr.DisableBorder = true
	qrData, err := qr.PNG(size)
	if err != nil {
		return nil, fmt.Errorf("生成 PNG 数据失败: %w", err)
	}

	return qrData, nil
}
