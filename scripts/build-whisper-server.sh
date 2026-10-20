#!/bin/sh
# Builds whisper-server as one static binary (Metal embedded, no dylibs) into
# resources/darwin/bin/, which `mygo build` copies into the .app and signs.
# The tag is the version the app was developed against.
set -eu
TAG="${WHISPER_TAG:-v1.9.2}"
OUT="$(cd "$(dirname "$0")/.." && pwd)/resources/darwin/bin"
SRC="$(mktemp -d)"
trap 'rm -rf "$SRC"' EXIT

git clone --quiet --depth 1 --branch "$TAG" https://github.com/ggml-org/whisper.cpp "$SRC/whisper.cpp"
cmake -S "$SRC/whisper.cpp" -B "$SRC/build" -DCMAKE_BUILD_TYPE=Release \
  -DBUILD_SHARED_LIBS=OFF -DGGML_METAL=ON -DGGML_METAL_EMBED_LIBRARY=ON \
  -DGGML_NATIVE=OFF -DCMAKE_OSX_ARCHITECTURES=arm64 -DCMAKE_OSX_DEPLOYMENT_TARGET=14.2 \
  -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_EXAMPLES=ON
cmake --build "$SRC/build" --config Release --target whisper-server -j "$(sysctl -n hw.ncpu)"
mkdir -p "$OUT"
cp "$SRC/build/bin/whisper-server" "$OUT/whisper-server"
strip -x "$OUT/whisper-server"
echo "built $OUT/whisper-server"
otool -L "$OUT/whisper-server"
