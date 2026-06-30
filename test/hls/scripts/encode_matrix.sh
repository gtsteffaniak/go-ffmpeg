#!/usr/bin/env bash
set -euo pipefail
INPUT="${1:-/home/graham/git/test-ffmpeg/.fixtures/h264_aac_mp4.mp4}"
RENDER="${RENDER:-/dev/dri/renderD128}"
RUNS="${RUNS:-3}"

median_ms() {
  python3 - "$@" <<'PY'
import sys, statistics
vals = [float(x) for x in sys.argv[1:]]
print(f"{statistics.median(vals):.0f}")
PY
}

bench() {
  local name="$1"
  shift
  local -a samples=()
  local i start end ms
  for ((i=0; i<RUNS; i++)); do
    start=$(python3 -c 'import time; print(time.time())')
    ffmpeg -hide_banner -nostats -loglevel error -y "$@" 2>/dev/null
    end=$(python3 -c 'import time; print(time.time())')
    ms=$(python3 -c "print((float('$end')-float('$start'))*1000)")
    samples+=("$ms")
  done
  ms=$(median_ms "${samples[@]}")
  printf '%-34s %6sms  [%s]\n' "$name" "$ms" "$(IFS=,; echo "${samples[*]}")"
}

hls_like() {
  local name="$1"
  shift
  bench "$name" "$@" \
    -movflags empty_moov+frag_keyframe+default_base_moof \
    -f mp4 /dev/null
}

run_block() {
  local label="$1" ss="$2"
  echo "=== $label ss=${ss}s (${RUNS} runs, HLS fMP4 output) ==="
  local seek=()
  if [[ "$ss" != "0" ]]; then seek=(-ss "$ss"); fi
  local common_end=(
    -map 0:v:0 -map 0:a:0? -sn -dn
    -b:v 2M -maxrate 2M -g 120
    -c:a aac -ar 48000 -ac 2
    -t 4
  )

  hls_like "qsv-cur-dec+cpu-scale" \
    -init_hw_device qsv=hw -c:v h264_qsv \
    "${seek[@]}" -i "$INPUT" \
    "${common_end[@]}" \
    -vf 'scale=-2:min(1080\,ih),format=nv12' \
    -c:v h264_qsv -preset 7 -pix_fmt nv12 -look_ahead_depth 0 -async_depth 1 -bf 2

  hls_like "qsv-swdec-bf2" \
    "${seek[@]}" -i "$INPUT" \
    "${common_end[@]}" \
    -vf 'scale=-2:min(1080\,ih),format=nv12' \
    -init_hw_device qsv=hw -c:v h264_qsv -preset 7 -pix_fmt nv12 -look_ahead_depth 0 -async_depth 1 -bf 2

  hls_like "qsv-swdec-bf0" \
    "${seek[@]}" -i "$INPUT" \
    "${common_end[@]}" \
    -vf 'scale=-2:min(1080\,ih),format=nv12' \
    -init_hw_device qsv=hw -c:v h264_qsv -preset 7 -pix_fmt nv12 -look_ahead_depth 0 -async_depth 1 -bf 0

  hls_like "qsv-swdec-preset7-ext" \
    "${seek[@]}" -i "$INPUT" \
    "${common_end[@]}" \
    -vf 'scale=-2:min(1080\,ih),format=nv12' \
    -init_hw_device qsv=hw -c:v h264_qsv -preset 7 -pix_fmt nv12 \
    -look_ahead_depth 0 -async_depth 1 -bf 0 -low_power 1

  hls_like "vaapi-hwdec+hwenc" \
    -init_hw_device vaapi=va:"$RENDER" -hwaccel vaapi -hwaccel_device va \
    "${seek[@]}" -i "$INPUT" \
    "${common_end[@]}" \
    -filter_hw_device va -vf 'scale=-2:min(1080\,ih),format=nv12,hwupload' \
    -c:v h264_vaapi

  hls_like "vaapi-swdec+hwenc" \
    "${seek[@]}" -i "$INPUT" \
    "${common_end[@]}" \
    -init_hw_device vaapi=va:"$RENDER" -filter_hw_device va \
    -vf 'scale=-2:min(1080\,ih),format=nv12,hwupload' \
    -c:v h264_vaapi

  hls_like "sw-x264-veryfast" \
    "${seek[@]}" -i "$INPUT" \
    "${common_end[@]}" \
    -vf 'scale=-2:min(1080\,ih)' \
    -c:v libx264 -preset veryfast -tune zerolatency -bf 0 -bufsize 2M

  hls_like "sw-x264-ultrafast" \
    "${seek[@]}" -i "$INPUT" \
    "${common_end[@]}" \
    -vf 'scale=-2:min(1080\,ih)' \
    -c:v libx264 -preset ultrafast -tune zerolatency -bf 0 -bufsize 2M

  echo
}

run_block seg0 0
run_block seg5 20

echo "=== init overhead ==="
bench "qsv-init+1frame" -init_hw_device qsv=hw -f lavfi -i nullsrc=s=640x360:d=0.04 -frames:v 1 -c:v h264_qsv -f null -
bench "vaapi-init+1frame" -init_hw_device vaapi=va:"$RENDER" -filter_hw_device va -f lavfi -i nullsrc=s=640x360:d=0.04 -vf format=nv12,hwupload -frames:v 1 -c:v h264_vaapi -f null -
