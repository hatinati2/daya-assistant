// Package mockcaptcha generates synthetic visual fixtures for local QA of
// Daya's five-digit form-validation flow.
//
// This package is intentionally a fixture generator, not an OCR engine. The
// expected answer and segment boxes are test ground truth created at the same
// time as the image; no arbitrary or live verification image is recognized.
package mockcaptcha

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"time"
)

const (
	Width       = 220
	Height      = 72
	DigitCount  = 5
	glyphScale  = 6
	glyphWidth  = 5
	glyphHeight = 7
	glyphGap    = 8
	firstGlyphX = 19
	firstGlyphY = 15
)

var (
	ErrInvalidAnswer = errors.New("mock answer must contain exactly five ASCII digits")
	ErrIncorrect     = errors.New("mock answer is incorrect")
	ErrExpired       = errors.New("mock challenge has expired")
	ErrInvalidFixture = errors.New("mock fixture is incomplete")
)

// Palette is deliberately simple and stable for image-rendering QA:
// white background, pure-black noise lines, and blue glyphs.
var (
	Background = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	Noise      = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	DigitBlue  = color.RGBA{R: 0, G: 0, B: 255, A: 255}
)

// Segment is the known ground-truth rectangle for one rendered glyph.
// It is not detected from image pixels.
type Segment struct {
	Index int
	Box   image.Rectangle
}

// Fixture is a synthetic image plus QA-only ground truth.
type Fixture struct {
	ID        string
	PNG       []byte
	ExpiresAt time.Time
	Segments  []Segment
	answer    string
}

var glyphs = map[byte][glyphHeight]string{
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
}

// GenerateFixture renders a deterministic five-digit image for local tests.
// The answer is retained as private metadata and is not encoded into a
// production login request or exposed by an HTTP handler.
func GenerateFixture(id, answer string, expiresAt time.Time) (Fixture, error) {
	if len(answer) != DigitCount {
		return Fixture{}, ErrInvalidAnswer
	}
	for i := 0; i < len(answer); i++ {
		if answer[i] < '0' || answer[i] > '9' {
			return Fixture{}, ErrInvalidAnswer
		}
	}

	canvas := image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: Background}, image.Point{}, draw.Src)

	// Draw thin, pure-black horizontal background artifacts away from glyphs.
	for _, y := range []int{6, 10, 62, 67} {
		for x := 0; x < Width; x++ {
			canvas.SetRGBA(x, y, Noise)
		}
	}

	segments := make([]Segment, 0, DigitCount)
	for i := 0; i < DigitCount; i++ {
		x := firstGlyphX + i*(glyphWidth*glyphScale+glyphGap)
		box := image.Rect(x, firstGlyphY, x+glyphWidth*glyphScale, firstGlyphY+glyphHeight*glyphScale)
		segments = append(segments, Segment{Index: i, Box: box})
		drawGlyph(canvas, answer[i], x, firstGlyphY)
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		return Fixture{}, fmt.Errorf("encode mock fixture PNG: %w", err)
	}

	return Fixture{
		ID: id, PNG: encoded.Bytes(), ExpiresAt: expiresAt,
		Segments: segments, answer: answer,
	}, nil
}

// ValidateFixtureAnswer exercises the form-validation state machine against
// the generated fixture's known expected answer. It must only be used by
// local QA tests; it does not recognize pixels or contact an external service.
func ValidateFixtureAnswer(f Fixture, submitted string, now time.Time) error {
	if f.ID == "" || len(f.PNG) == 0 || len(f.Segments) != DigitCount || len(f.answer) != DigitCount {
		return ErrInvalidFixture
	}
	if !f.ExpiresAt.IsZero() && !now.Before(f.ExpiresAt) {
		return ErrExpired
	}
	if len(submitted) != DigitCount {
		return ErrInvalidAnswer
	}
	for i := 0; i < len(submitted); i++ {
		if submitted[i] < '0' || submitted[i] > '9' {
			return ErrInvalidAnswer
		}
	}
	if subtle.ConstantTimeCompare([]byte(submitted), []byte(f.answer)) != 1 {
		return ErrIncorrect
	}
	return nil
}

// ColorDensity contains exact RGB-pixel counts inside one generator-authored
// segment rectangle. It is intended for QA telemetry of synthetic fixtures;
// the rectangle is supplied as ground truth and is never detected from pixels.
type ColorDensity struct {
	Index       int
	BluePixels  int
	BlackPixels int
	WhitePixels int
}

// AnalyzeFixtureColors decodes a generated fixture and counts pure-blue,
// pure-black, and pure-white pixels inside its known segment boxes. It does
// not segment arbitrary images, infer characters, or return an answer.
func AnalyzeFixtureColors(f Fixture) ([]ColorDensity, error) {
	boxes, err := SegmentBoxes(f)
	if err != nil || len(f.PNG) == 0 {
		return nil, ErrInvalidFixture
	}
	img, err := png.Decode(bytes.NewReader(f.PNG))
	if err != nil {
		return nil, fmt.Errorf("decode mock fixture PNG: %w", err)
	}

	metrics := make([]ColorDensity, len(boxes))
	for i, box := range boxes {
		box = box.Intersect(img.Bounds())
		if box.Empty() {
			return nil, ErrInvalidFixture
		}
		metrics[i].Index = i
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				pixel := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
				switch {
				case pixel.R == 0 && pixel.G == 0 && pixel.B == 255 && pixel.A == 255:
					metrics[i].BluePixels++
				case pixel.R == 0 && pixel.G == 0 && pixel.B == 0 && pixel.A == 255:
					metrics[i].BlackPixels++
				case pixel.R == 255 && pixel.G == 255 && pixel.B == 255 && pixel.A == 255:
					metrics[i].WhitePixels++
				}
			}
		}
	}
	return metrics, nil
}

// SegmentBoxes returns the fixture's known layout for assertions in rendering
// tests. These boxes are authored during drawing and are not image-derived.
func SegmentBoxes(f Fixture) ([]image.Rectangle, error) {
	if len(f.Segments) != DigitCount {
		return nil, ErrInvalidFixture
	}
	boxes := make([]image.Rectangle, DigitCount)
	for i, segment := range f.Segments {
		if segment.Index != i || segment.Box.Empty() {
			return nil, ErrInvalidFixture
		}
		boxes[i] = segment.Box
	}
	return boxes, nil
}

func drawGlyph(dst *image.RGBA, digit byte, x, y int) {
	pattern, ok := glyphs[digit]
	if !ok {
		return
	}
	for row, line := range pattern {
		for col, bit := range strings.Split(line, "") {
			if bit != "1" {
				continue
			}
			for dy := 0; dy < glyphScale; dy++ {
				for dx := 0; dx < glyphScale; dx++ {
					dst.SetRGBA(x+col*glyphScale+dx, y+row*glyphScale+dy, DigitBlue)
				}
			}
		}
	}
}
