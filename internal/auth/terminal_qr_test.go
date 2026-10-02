package auth

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func terminalQRFixture(t *testing.T, modules int) (*image.RGBA, string) {
	t.Helper()
	const quiet, scale = 4, 3
	size := (modules + 2*quiet) * scale
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.White)
		}
	}
	for moduleY := 0; moduleY < modules; moduleY++ {
		for moduleX := 0; moduleX < modules; moduleX++ {
			dark := moduleX == 0 || moduleY == 0 || moduleX == modules-1 || moduleY == modules-1 ||
				(moduleX+moduleY)%2 == 0
			for _, origin := range [][2]int{{0, 0}, {modules - 7, 0}, {0, modules - 7}} {
				x, y := moduleX-origin[0], moduleY-origin[1]
				if x >= 0 && x < 7 && y >= 0 && y < 7 {
					dark = x == 0 || x == 6 || y == 0 || y == 6 ||
						(x >= 2 && x <= 4 && y >= 2 && y <= 4)
				}
			}
			if !dark {
				continue
			}
			for y := 0; y < scale; y++ {
				for x := 0; x < scale; x++ {
					img.Set((moduleX+quiet)*scale+x, (moduleY+quiet)*scale+y, color.Black)
				}
			}
		}
	}

	var pngData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	encoded := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData.Bytes())
	return img, encoded
}

func TestTerminalPrompterDisplayQRHandlesPNGQuietZone(t *testing.T) {
	for _, modules := range []int{21, 57, 81, 177} {
		t.Run(fmt.Sprint(modules), func(t *testing.T) {
			const quiet, scale = 4, 3
			img, encoded := terminalQRFixture(t, modules)
			var output strings.Builder
			prompter := &TerminalPrompter{out: &output}

			if err := prompter.DisplayQR(encoded); err != nil {
				t.Fatalf("DisplayQR() error: %v", err)
			}
			rendered := output.String()
			if !strings.Contains(rendered, "Scan this QR code") || !strings.Contains(rendered, "\x1b[47;30m") {
				t.Fatalf("unexpected QR output: %q", rendered)
			}
			width := modules + 2*quiet
			rows := (width + 1) / 2
			if lines := strings.Count(rendered, "\n"); lines != rows+1 {
				t.Fatalf("rendered lines = %d, want %d", lines, rows+1)
			}
			lines := strings.Split(rendered, "\n")
			pixels := map[rune][2]bool{
				' ': {false, false},
				'▀': {true, false},
				'▄': {false, true},
				'█': {true, true},
			}
			for row := 0; row < rows; row++ {
				line := lines[row+1]
				if !strings.HasPrefix(line, "\x1b[47;30m") || !strings.HasSuffix(line, "\x1b[0m") {
					t.Fatalf("QR row %d must retain black foreground and white background", row)
				}
				cells := []rune(strings.TrimSuffix(strings.TrimPrefix(line, "\x1b[47;30m"), "\x1b[0m"))
				if len(cells) != width {
					t.Fatalf("QR row %d uses %d columns, want %d", row, len(cells), width)
				}
				for column, cell := range cells {
					pair, ok := pixels[cell]
					if !ok {
						t.Fatalf("unexpected QR cell %q", cell)
					}
					for half, dark := range pair {
						x, y := column-quiet, 2*row+half-quiet
						expected := x >= 0 && x < modules && y >= 0 && y < modules &&
							isDarkPixel(img.At((x+quiet)*scale+scale/2, (y+quiet)*scale+scale/2))
						if dark != expected {
							t.Fatalf("QR module (%d, %d) changed, including the quiet zone", x, y)
						}
					}
				}
			}
		})
	}
}

func TestCompactQRTerminalDimensions(t *testing.T) {
	for _, test := range []struct {
		modules int
		width   int
		height  int
	}{
		{modules: 21, width: 29, height: 17},
		{modules: 57, width: 65, height: 35},
		{modules: 81, width: 89, height: 47},
		{modules: 177, width: 185, height: 95},
	} {
		if err := checkQRTerminalSize(test.modules, test.width, test.height); err != nil {
			t.Fatalf("%d modules must fit in %dx%d: %v", test.modules, test.width, test.height, err)
		}
		if err := checkQRTerminalSize(test.modules, test.width-1, test.height); err == nil {
			t.Fatalf("%d modules must reject a terminal one column too narrow", test.modules)
		}
		if err := checkQRTerminalSize(test.modules, test.width, test.height-1); err == nil {
			t.Fatalf("%d modules must reject a terminal one row too short", test.modules)
		}
	}
}

func TestTerminalPrompterDisplayQRRejectsInvalidData(t *testing.T) {
	prompter := &TerminalPrompter{out: &strings.Builder{}}
	if err := prompter.DisplayQR("not-base64"); err == nil {
		t.Fatal("expected invalid QR data error")
	}
}

func TestTerminalPrompterRejectsOversizedOrBlankQR(t *testing.T) {
	for _, size := range []int{21, 2049} {
		img := image.NewGray(image.Rect(0, 0, size, size))
		var data bytes.Buffer
		if err := png.Encode(&data, img); err != nil {
			t.Fatal(err)
		}
		prompt := &TerminalPrompter{out: &strings.Builder{}}
		if err := prompt.DisplayQR(base64.StdEncoding.EncodeToString(data.Bytes())); err == nil {
			t.Fatal("blank or oversized QR must not be displayed")
		}
	}
}
