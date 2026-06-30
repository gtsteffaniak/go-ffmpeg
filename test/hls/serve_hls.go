package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func runServeHLS(args []string) int {
	fs := flag.NewFlagSet("serve-hls", flag.ExitOnError)
	file := fs.String("file", envOr("HLS_TEST_FILE", defaultSampleVideo()), "input media file")
	segments := fs.Int("segments", 8, "segments to encode and serve")
	mode := fs.String("mode", "remux", "encode mode")
	outDir := fs.String("out", ".playback-cache", "output directory for segments + playlist")
	port := fs.Int("port", 8765, "HTTP port")
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
		fmt.Fprintln(os.Stderr, "timeline validation failed before serve")
		return 1
	}

	if len(encoded.Init) > 0 {
		if err := os.WriteFile(filepath.Join(*outDir, "init.m4s"), encoded.Init, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write init: %v\n", err)
			return 1
		}
	}
	durs := make([]float64, len(encoded.Segments))
	for i, seg := range encoded.Segments {
		name := fmt.Sprintf("seg%d.m4s", i)
		if err := os.WriteFile(filepath.Join(*outDir, name), seg.Media, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write segment %d: %v\n", i, err)
			return 1
		}
		durs[i] = seg.Report.ActualDurSec
	}
	if err := writeM3U8(filepath.Join(*outDir, "playlist.m3u8"), len(encoded.Segments), durs); err != nil {
		fmt.Fprintf(os.Stderr, "write playlist: %v\n", err)
		return 1
	}

	addr := fmt.Sprintf(":%d", *port)
	playbackURL := fmt.Sprintf("http://127.0.0.1:%d/playback/index.html?playlist=/media/playlist.m3u8&duration=20", *port)
	fmt.Printf("\nServing HLS on http://127.0.0.1%s\n", addr)
	fmt.Printf("Playback test (hls.js only): %s\n", playbackURL)
	fmt.Printf("Segments in %s\n", *outDir)
	fmt.Println("After ~20s open DevTools → window.__playbackAudit (jumps should be [])")
	fmt.Println("Automated: make playback-test")

	mux := http.NewServeMux()
	mux.Handle("/playback/", http.StripPrefix("/playback/", http.FileServer(http.Dir("playback"))))
	mux.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(*outDir))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, playbackURL, http.StatusFound)
	})

	return runHTTPServer(addr, mux)
}

func writeM3U8(path string, segCount int, durs []float64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprint(f, "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-INDEPENDENT-SEGMENTS\n#EXT-X-MAP:URI=\"init.m4s\"\n")
	for i := 0; i < segCount; i++ {
		d := 4.0
		if i < len(durs) && durs[i] > 0 {
			d = durs[i]
		}
		fmt.Fprintf(f, "#EXTINF:%.3f,\nseg%d.m4s\n", d, i)
	}
	fmt.Fprint(f, "#EXT-X-ENDLIST\n")
	return nil
}

func runHTTPServer(addr string, mux http.Handler) int {
	srv := &http.Server{Addr: addr, Handler: mux}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		return 1
	}
	return 0
}
