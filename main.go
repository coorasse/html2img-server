package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

var CHROME_ARGS = []string{
	"--headless",
	"--hide-scrollbars",
	"--window-size=600,314",
	"--disable-extensions",
	"--no-sandbox",
	"--disable-background-networking",
	"--disable-cache",
	"--force-device-scale-factor=2",
}

const minScreenshotBytes = 1024

func chromeTimeout() time.Duration {
	if raw := os.Getenv("CHROME_TIMEOUT_SECONDS"); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return 10 * time.Second
}

func takeScreenshot(html []byte) ([]byte, error) {
	// Chrome leaves its profile, disk cache and temp files behind on every run, so each render gets its own
	// directory that is removed afterwards.
	workDir, err := os.MkdirTemp("", "html2img-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)

	htmlPath := filepath.Join(workDir, "page.html")
	if err := os.WriteFile(htmlPath, html, 0600); err != nil {
		return nil, err
	}

	screenshotPath := filepath.Join(workDir, "screenshot.png")

	params := append(CHROME_ARGS,
		fmt.Sprintf("--user-data-dir=%s", filepath.Join(workDir, "profile")),
		fmt.Sprintf("file://%s", htmlPath),
		fmt.Sprintf("--screenshot=%s", screenshotPath),
	)

	timeout := chromeTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, chromeExecutable(), params...)
	cmd.Env = append(os.Environ(),
		"TMPDIR="+workDir,
		"XDG_CACHE_HOME="+filepath.Join(workDir, "cache"),
		"XDG_CONFIG_HOME="+filepath.Join(workDir, "config"),
	)
	// Kill the whole process group on timeout, otherwise Chrome helper processes outlive the render.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	log.Println("Running command: ", cmd.String())

	start := time.Now()
	err = cmd.Run()
	elapsed := time.Since(start)
	log.Printf("Time taken: %s", elapsed)

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("chrome timed out after %s", timeout)
	}
	if err != nil {
		return nil, err
	}

	screenshot, err := os.ReadFile(screenshotPath)
	if err != nil {
		return nil, err
	}

	if err := validateScreenshot(screenshot); err != nil {
		return nil, err
	}

	return screenshot, nil
}

func validateScreenshot(data []byte) error {
	if len(data) < minScreenshotBytes {
		return fmt.Errorf("screenshot is too small (%d bytes)", len(data))
	}

	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("screenshot is not a decodable image: %w", err)
	}

	return nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	apiToken := os.Getenv("API_TOKEN")
	if token != apiToken {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Body == nil {
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	html, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	result, err := takeScreenshot(html)
	if err != nil {
		log.Println("takeScreenshot failed:", err)
		http.Error(w, "Error while taking screenshot", http.StatusInternalServerError)
		return
	}

	_, err = w.Write(result)
	if err != nil {
		http.Error(w, "Error writing response", http.StatusInternalServerError)
		return
	}
}

func chromeExecutable() string {
	chrome := os.Getenv("CHROME_BIN")
	_, err := exec.LookPath(chrome)
	if err == nil {
		return chrome
	}

	log.Fatal("Google Chrome not found")
	return ""
}

func main() {
	// check if chrome is installed
	chromeExecutable()

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(os.Getenv("APP_PORT"), nil))
}
