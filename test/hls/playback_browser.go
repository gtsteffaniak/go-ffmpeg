package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func runPlaybackTest(args []string) int {
	fs := flag.NewFlagSet("playback-test", flag.ExitOnError)
	file := fs.String("file", envOr("HLS_TEST_FILE", defaultSampleVideo()), "input media file")
	segments := fs.Int("segments", 8, "segments to encode")
	mode := fs.String("mode", "remux", "encode mode")
	duration := fs.Int("duration", 15, "seconds of playback to sample")
	outDir := fs.String("out", ".playback-cache", "segment cache directory")
	debug := fs.Bool("debug", false, "ffmpeg stderr")
	_ = fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	svc, err := initFFmpeg(ctx, *debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg init: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		return 1
	}

	encoded, err := encodeHLS(ctx, svc, *file, *mode, *segments, 0.05)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		return 1
	}
	printHumanReport(encoded.Report)
	if !encoded.Report.Pass {
		return 1
	}
	if len(encoded.Init) > 0 {
		_ = os.WriteFile(filepath.Join(*outDir, "init.m4s"), encoded.Init, 0o644)
	}
	durs := make([]float64, len(encoded.Segments))
	for i, seg := range encoded.Segments {
		_ = os.WriteFile(filepath.Join(*outDir, fmt.Sprintf("seg%d.m4s", i)), seg.Media, 0o644)
		durs[i] = seg.Report.ActualDurSec
	}
	if err := writeM3U8(filepath.Join(*outDir, "playlist.m3u8"), len(encoded.Segments), durs); err != nil {
		fmt.Fprintf(os.Stderr, "playlist: %v\n", err)
		return 1
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		return 1
	}
	port := ln.Addr().(*net.TCPAddr).Port
	mux := http.NewServeMux()
	mux.Handle("/playback/", http.StripPrefix("/playback/", http.FileServer(http.Dir("playback"))))
	mux.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(*outDir))))
	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(ln)
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	playURL := fmt.Sprintf("%s/playback/index.html?playlist=/media/playlist.m3u8&duration=%d&autoplay=1", baseURL, *duration)
	fmt.Printf("Running browser playback test: %s\n", playURL)

	if _, err := exec.LookPath("npx"); err != nil {
		fmt.Fprintln(os.Stderr, "npx not found — open the URL above manually and inspect window.__playbackAudit")
		return 2
	}

	playbackDir, err := filepath.Abs("playback")
	if err != nil {
		fmt.Fprintf(os.Stderr, "playback dir: %v\n", err)
		return 1
	}
	if err := ensurePlaywright(playbackDir); err != nil {
		fmt.Fprintf(os.Stderr, "playwright setup: %v\n", err)
		return 1
	}

	cmd := exec.Command("npx", "playwright", "test", "playwright.spec.mjs",
		"--config=playwright.config.mjs",
		"--grep=smooth playback",
	)
	cmd.Dir = playbackDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PLAYBACK_TEST_URL=%s", playURL),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "playwright: %v\n", err)
		return 1
	}
	fmt.Println("PASS — browser playback had no playhead jumps")
	return 0
}

func ensurePlaywright(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "node_modules", "@playwright", "test")); err == nil {
		return nil
	}
	fmt.Println("Installing @playwright/test in playback/ (first run only)...")
	install := exec.Command("npm", "install", "--no-fund", "--no-audit")
	install.Dir = dir
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("npm install: %w", err)
	}
	browsers := exec.Command("npx", "playwright", "install", "chromium")
	browsers.Dir = dir
	browsers.Stdout = os.Stdout
	browsers.Stderr = os.Stderr
	if err := browsers.Run(); err != nil {
		return fmt.Errorf("playwright install chromium: %w", err)
	}
	return nil
}
