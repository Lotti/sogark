package auth

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"

	"golang.org/x/term"
)

func (p *TerminalPrompter) DisplayQR(encoded string) error {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return errors.New("CyberArk returned an empty QR image")
	}
	if comma := strings.IndexByte(encoded, ','); strings.HasPrefix(encoded, "data:image/") && comma >= 0 {
		encoded = encoded[comma+1:]
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("decode QR image: %w", err)
	}
	size, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("inspect QR PNG: %w", err)
	}
	if size.Width != size.Height || size.Width < 21 || size.Width > 2048 {
		return errors.New("QR PNG must be square and between 21 and 2048 pixels")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode QR PNG: %w", err)
	}

	grid, ok := inferQRGrid(img)
	if !ok {
		return fmt.Errorf("cannot infer QR module grid from %dx%d image", img.Bounds().Dx(), img.Bounds().Dy())
	}

	var rendered strings.Builder
	rendered.WriteString("[*] Scan this QR code:\n")
	if output, ok := p.out.(*os.File); ok && term.IsTerminal(int(output.Fd())) {
		width, height, err := term.GetSize(int(output.Fd()))
		if err != nil {
			return fmt.Errorf("read QR terminal size: %w", err)
		}
		if err := checkQRTerminalSize(grid.modules, width, height); err != nil {
			return err
		}
	}
	darkModule := func(x, y int) bool {
		if x < 0 || x >= grid.modules || y < 0 || y >= grid.modules {
			return false
		}
		px := grid.bounds.Min.X + (2*x+1)*grid.bounds.Dx()/(2*grid.modules)
		py := grid.bounds.Min.Y + (2*y+1)*grid.bounds.Dy()/(2*grid.modules)
		return isDarkPixel(img.At(px, py))
	}
	for y := -qrQuietZone; y < grid.modules+qrQuietZone; y += 2 {
		rendered.WriteString("\x1b[47;30m")
		for x := -qrQuietZone; x < grid.modules+qrQuietZone; x++ {
			top, bottom := darkModule(x, y), darkModule(x, y+1)
			switch {
			case top && bottom:
				rendered.WriteRune('█')
			case top:
				rendered.WriteRune('▀')
			case bottom:
				rendered.WriteRune('▄')
			default:
				rendered.WriteByte(' ')
			}
		}
		rendered.WriteString("\x1b[0m\n")
	}
	return writeTerminalQR(p.out, rendered.String())
}

const qrQuietZone = 4

func checkQRTerminalSize(modules, width, height int) error {
	requiredWidth := modules + 2*qrQuietZone
	requiredHeight := (requiredWidth+1)/2 + 2
	if (width > 0 && width < requiredWidth) || (height > 0 && height < requiredHeight) {
		return fmt.Errorf("terminal is too small for this QR; enlarge it or reduce the font size to provide at least %d columns and %d rows, then retry", requiredWidth, requiredHeight)
	}
	return nil
}

type qrGrid struct {
	bounds  image.Rectangle
	modules int
}

func inferQRGrid(img image.Image) (qrGrid, bool) {
	bounds, ok := darkPixelBounds(img)
	if !ok {
		return qrGrid{}, false
	}
	width, height := bounds.Dx(), bounds.Dy()
	scale := 0
	addRun := func(length int) {
		if length <= 0 {
			return
		}
		if scale == 0 {
			scale = length
			return
		}
		scale = greatestCommonDivisor(scale, length)
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		last := isDarkPixel(img.At(bounds.Min.X, y))
		run := 1
		for x := bounds.Min.X + 1; x < bounds.Max.X; x++ {
			current := isDarkPixel(img.At(x, y))
			if current == last {
				run++
				continue
			}
			addRun(run)
			last, run = current, 1
		}
		addRun(run)
	}
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		last := isDarkPixel(img.At(x, bounds.Min.Y))
		run := 1
		for y := bounds.Min.Y + 1; y < bounds.Max.Y; y++ {
			current := isDarkPixel(img.At(x, y))
			if current == last {
				run++
				continue
			}
			addRun(run)
			last, run = current, 1
		}
		addRun(run)
	}
	if scale > 0 && width%scale == 0 && height%scale == 0 {
		modules := width / scale
		if validQRModules(modules) && height/scale == modules && hasFinderPatterns(img, bounds, modules) {
			return qrGrid{bounds: bounds, modules: modules}, true
		}
	}

	bestModules, bestError := 0, int(^uint(0)>>1)
	for modules := 21; modules <= 177; modules += 4 {
		scaleX := max(1, (width+modules/2)/modules)
		scaleY := max(1, (height+modules/2)/modules)
		sizeError := absInt(width-modules*scaleX) + absInt(height-modules*scaleY)
		if scaleX == scaleY && sizeError < bestError && hasFinderPatterns(img, bounds, modules) {
			bestModules, bestError = modules, sizeError
		}
	}
	if bestError > 4 {
		return qrGrid{}, false
	}
	return qrGrid{bounds: bounds, modules: bestModules}, true
}

func hasFinderPatterns(img image.Image, bounds image.Rectangle, modules int) bool {
	for _, origin := range [][2]int{{0, 0}, {modules - 7, 0}, {0, modules - 7}} {
		for y := 0; y < 7; y++ {
			for x := 0; x < 7; x++ {
				expected := x == 0 || x == 6 || y == 0 || y == 6 ||
					(x >= 2 && x <= 4 && y >= 2 && y <= 4)
				px := bounds.Min.X + (2*(origin[0]+x)+1)*bounds.Dx()/(2*modules)
				py := bounds.Min.Y + (2*(origin[1]+y)+1)*bounds.Dy()/(2*modules)
				if isDarkPixel(img.At(px, py)) != expected {
					return false
				}
			}
		}
	}
	return true
}

func darkPixelBounds(img image.Image) (image.Rectangle, bool) {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X-1, bounds.Min.Y-1
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !isDarkPixel(img.At(x, y)) {
				continue
			}
			minX = min(minX, x)
			minY = min(minY, y)
			maxX = max(maxX, x)
			maxY = max(maxY, y)
		}
	}
	if maxX < minX || maxY < minY {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}

func validQRModules(modules int) bool {
	return modules >= 21 && modules <= 177 && (modules-21)%4 == 0
}

func greatestCommonDivisor(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func isDarkPixel(c color.Color) bool {
	r, g, b, a := c.RGBA()
	return r+g+b+3*(65535-a) < 3*32768
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
