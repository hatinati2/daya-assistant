# Mock CAPTCHA QA fixtures

This package creates deterministic, local-only PNG fixtures for testing the
five-digit answer field and form-validation state machine.

- Canvas: 220 × 72 pixels.
- Background: pure white.
- Synthetic digits: pure blue, rendered from a small built-in bitmap font.
- Noise: thin, pure-black horizontal lines outside the digit glyphs.
- Ground truth: five equal, left-to-right rectangles recorded when the image
  is drawn.
- Expected answer: held as private fixture metadata for unit tests.

Run the isolated tests with:

```sh
go test ./internal/mockcaptcha
```

This is intentionally not an OCR pipeline: it does not accept arbitrary images,
perform color-based digit recognition, infer text from pixels, or integrate
with the real Daya login/captcha endpoints. Tests obtain the expected answer
from fixture metadata, so the validation state machine can be exercised
without a third-party service or a live challenge.
