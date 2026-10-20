#!/bin/sh
# Copies the sherpa-onnx dylibs into resources/darwin/lib/, which `mygo build`
# puts in Contents/Resources/lib. The app finds them through the rpath that
# the build scripts of package.json add (@executable_path/../Resources/lib).
set -eu
cd "$(dirname "$0")/.."
DIR="$(go list -m -f '{{.Dir}}' github.com/k2-fsa/sherpa-onnx-go-macos)/lib/aarch64-apple-darwin"
mkdir -p resources/darwin/lib
cp "$DIR/libsherpa-onnx-c-api.dylib" "$DIR/libonnxruntime.dylib" resources/darwin/lib/
chmod u+w resources/darwin/lib/*.dylib
echo "copied to resources/darwin/lib"
