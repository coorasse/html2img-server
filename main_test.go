package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHandler(t *testing.T) {
	html := []byte(`<html>
		<head><style>body {background-color: yellow;width: 600px;height: 310px;}</style></head>
		<body><h1>Hello from Go!</h1></body>
		</html>`)

	apiToken := os.Getenv("API_TOKEN")
	req, err := http.NewRequest("POST", url(apiToken), bytes.NewBuffer(html))
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	screenshot, err := ioutil.ReadAll(rr.Body)
	if err != nil {
		t.Fatalf("Error reading response body: %v", err)
	}

	if len(screenshot) == 0 {
		t.Errorf("handler returned an empty response")
	}

	img, _, err := image.Decode(bytes.NewBuffer(screenshot))
	if err != nil {
		t.Fatalf("Error decoding screenshot: %v", err)
	}

	zoomFactor := 2
	expectedWidth := 600 * zoomFactor
	expectedHeight := 314 * zoomFactor
	if img.Bounds().Dx() != expectedWidth || img.Bounds().Dy() != expectedHeight {
		t.Errorf("Unexpected screenshot dimensions: got %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), expectedWidth, expectedHeight)
	}

	if err := ioutil.WriteFile("result.png", screenshot, 0644); err != nil {
		t.Fatalf("Error writing result to file: %v", err)
	}
}

func TestTakeScreenshotCleansUp(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	html := []byte(`<html><body style="background: yellow"><h1>Hello from Go!</h1></body></html>`)
	if _, err := takeScreenshot(html); err != nil {
		t.Fatalf("takeScreenshot failed: %v", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Error reading temp dir: %v", err)
	}
	for _, entry := range entries {
		t.Errorf("leftover in temp dir: %s", entry.Name())
	}
}

func TestValidateScreenshot(t *testing.T) {
	t.Run("rejects an empty screenshot", func(t *testing.T) {
		if err := validateScreenshot([]byte{}); err == nil {
			t.Error("expected an error for an empty screenshot")
		}
	})

	t.Run("rejects a non-image byte blob", func(t *testing.T) {
		junk := make([]byte, minScreenshotBytes)
		if err := validateScreenshot(junk); err == nil {
			t.Error("expected an error for a non-image byte blob")
		}
	})

	t.Run("accepts a valid screenshot", func(t *testing.T) {
		valid := checkeredPNG(t)
		if err := validateScreenshot(valid); err != nil {
			t.Errorf("expected no error for a valid screenshot, got %v", err)
		}
	})
}

func checkeredPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 1200, 628))
	for y := 0; y < 628; y++ {
		for x := 0; x < 1200; x++ {
			if (x/20+y/20)%2 == 0 {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.White)
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}

	return buf.Bytes()
}

func TestUnauthorized(t *testing.T) {
	req, _ := http.NewRequest("POST", url("invalid-token"), nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}

func TestEmptyRequest(t *testing.T) {
	apiToken := os.Getenv("API_TOKEN")
	req, _ := http.NewRequest("POST", url(apiToken), nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func url(apiToken string) string {
	port := os.Getenv("APP_PORT")
	return "http://localhost" + port + "/?token=" + apiToken
}
