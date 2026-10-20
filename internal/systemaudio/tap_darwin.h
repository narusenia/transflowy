#ifndef TRANSFLOWY_TAP_H
#define TRANSFLOWY_TAP_H

#include <stdint.h>

typedef struct {
	double rate;     // sample rate of the delivered audio
	int channels;    // interleaved channels per frame
	char err[256];   // set when ta_start returns NULL
} ta_info;

// ta_start taps the system's output mix and calls goTapData on the audio
// thread with interleaved float32 frames. It returns NULL on failure.
void *ta_start(uintptr_t ctx, ta_info *info);
void ta_stop(void *state);

#endif
