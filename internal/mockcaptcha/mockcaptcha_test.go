package mockcaptcha

import (
	"bytes"
	"errors"
	"image/png"
	"testing"
	"time"
)

func TestGenerateFixtureRendersFiveBlueDigitsAndBlackNoise(t *testing.T) {
	now := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	fixture, err := GenerateFixture("qa-001", "50821", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("GenerateFixture: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(fixture.PNG))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if img.Bounds().Dx() != Width || img.Bounds().Dy() != Height {
		t.Fatalf("image size = %v, want %dx%d", img.Bounds(), Width, Height)
	}
	if got := img.At(20, 6); got != Noise {
		t.Fatalf("noise pixel = %#v, want pure black", got)
	}
	if got := img.At(firstGlyphX, firstGlyphY); got != DigitBlue {
		t.Fatalf("digit pixel = %#v, want blue", got)
	}
	if got := img.At(Width-2, Height-2); got != Background {
		t.Fatalf("background pixel = %#v, want white", got)
	}
	boxes, err := SegmentBoxes(fixture)
	if err != nil {
		t.Fatalf("SegmentBoxes: %v", err)
	}
	if len(boxes) != DigitCount {
		t.Fatalf("segment count = %d, want %d", len(boxes), DigitCount)
	}
	for i, box := range boxes {
		if box.Dx() != glyphWidth*glyphScale || box.Dy() != glyphHeight*glyphScale {
			t.Errorf("segment %d size = %v, want %dx%d", i, box.Size(), glyphWidth*glyphScale, glyphHeight*glyphScale)
		}
		if i > 0 && boxes[i-1].Min.X >= box.Min.X {
			t.Errorf("segments are not ordered left-to-right: %v then %v", boxes[i-1], box)
		}
	}
}

func TestValidateFixtureAnswer(t *testing.T) {
	now := time.Now()
	fixture, err := GenerateFixture("qa-002", "80417", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateFixtureAnswer(fixture, "80417", now); err != nil {
		t.Fatalf("valid answer rejected: %v", err)
	}
	if err := ValidateFixtureAnswer(fixture, "80418", now); !errors.Is(err, ErrIncorrect) {
		t.Fatalf("wrong answer error = %v, want ErrIncorrect", err)
	}
	if err := ValidateFixtureAnswer(fixture, "1234", now); !errors.Is(err, ErrInvalidAnswer) {
		t.Fatalf("short answer error = %v, want ErrInvalidAnswer", err)
	}
	if err := ValidateFixtureAnswer(fixture, "12A45", now); !errors.Is(err, ErrInvalidAnswer) {
		t.Fatalf("non-digit answer error = %v, want ErrInvalidAnswer", err)
	}
}

func TestValidateFixtureAnswerRejectsExpiredChallenge(t *testing.T) {
	now := time.Now()
	fixture, err := GenerateFixture("qa-expired", "12345", now.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateFixtureAnswer(fixture, "12345", now); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired fixture error = %v, want ErrExpired", err)
	}
}

func TestGenerateFixtureRejectsNonFiveDigitAnswers(t *testing.T) {
	for _, answer := range []string{"", "1234", "123456", "12a45", "۱۲۳۴۵"} {
		if _, err := GenerateFixture("bad", answer, time.Time{}); !errors.Is(err, ErrInvalidAnswer) {
			t.Errorf("GenerateFixture(%q) error = %v, want ErrInvalidAnswer", answer, err)
		}
	}
}
