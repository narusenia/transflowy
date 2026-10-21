package systemaudio

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=14.2
#cgo LDFLAGS: -framework CoreAudio -framework Foundation
#include <stdlib.h>
#include "tap_darwin.h"
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"unsafe"
)

// Supported reports whether this build can capture system audio.
const Supported = true

type tap struct {
	state  unsafe.Pointer
	handle cgo.Handle
}

//export goTapData
func goTapData(ctx C.uintptr_t, samples *C.float, frames, channels C.int, rate C.double) {
	fn := cgo.Handle(ctx).Value().(func([]float32, int, float64))
	fn(unsafe.Slice((*float32)(unsafe.Pointer(samples)), int(frames*channels)), int(channels), float64(rate))
}

// startTap calls fn on the audio thread with interleaved float32 frames of
// everything the Mac plays, and the rate they run at now. fn must copy what it keeps.
func startTap(fn func(frames []float32, channels int, rate float64)) (stop func(), rate float64, channels int, err error) {
	h := cgo.NewHandle(fn)
	var info C.ta_info
	state := C.ta_start(C.uintptr_t(h), &info)
	if state == nil {
		h.Delete()
		return nil, 0, 0, errors.New(C.GoString(&info.err[0]))
	}
	return func() {
		C.ta_stop(state)
		h.Delete()
	}, float64(info.rate), int(info.channels), nil
}
