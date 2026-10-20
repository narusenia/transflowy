// Core Audio process tap (macOS 14.2+): the mix of everything the Mac plays,
// before the volume knob. No virtual driver is installed.
#import <CoreAudio/CoreAudio.h>
#import <CoreAudio/AudioHardwareTapping.h>
#import <CoreAudio/CATapDescription.h>
#import <Foundation/Foundation.h>
#include <stdio.h>
#include "tap_darwin.h"

extern void goTapData(uintptr_t ctx, float *samples, int frames, int channels);

typedef struct {
	AudioObjectID tap;
	AudioObjectID agg;
	AudioDeviceIOProcID proc;
	uintptr_t ctx;
} ta_state;

static OSStatus ioproc(AudioObjectID dev, const AudioTimeStamp *now, const AudioBufferList *in,
                       const AudioTimeStamp *inTime, AudioBufferList *out, const AudioTimeStamp *outTime, void *client) {
	ta_state *st = client;
	if (in && in->mNumberBuffers > 0) {
		const AudioBuffer *b = &in->mBuffers[0];
		int ch = b->mNumberChannels > 0 ? b->mNumberChannels : 1;
		goTapData(st->ctx, (float *)b->mData, (int)(b->mDataByteSize / (sizeof(float) * ch)), ch);
	}
	return noErr;
}

static void fail(ta_info *info, const char *what, OSStatus s) {
	snprintf(info->err, sizeof info->err, "%s failed (%d)", what, (int)s);
}

void ta_stop(void *p) {
	ta_state *st = p;
	if (!st) return;
	if (st->proc) {
		AudioDeviceStop(st->agg, st->proc);
		AudioDeviceDestroyIOProcID(st->agg, st->proc);
	}
	if (st->agg) AudioHardwareDestroyAggregateDevice(st->agg);
	if (st->tap) AudioHardwareDestroyProcessTap(st->tap);
	free(st);
}

void *ta_start(uintptr_t ctx, ta_info *info) {
	@autoreleasepool {
		ta_state *st = calloc(1, sizeof *st);
		st->ctx = ctx;
		info->err[0] = 0;

		CATapDescription *d = [[CATapDescription alloc] initStereoGlobalTapButExcludeProcesses:@[]];
		d.name = @"transflowy";
		d.privateTap = YES;
		d.muteBehavior = CATapUnmuted;
		OSStatus s = AudioHardwareCreateProcessTap(d, &st->tap);
		if (s != noErr) { fail(info, "create tap", s); st->tap = 0; ta_stop(st); return NULL; }

		AudioStreamBasicDescription fmt;
		UInt32 size = sizeof fmt;
		AudioObjectPropertyAddress addr = {kAudioTapPropertyFormat, kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain};
		s = AudioObjectGetPropertyData(st->tap, &addr, 0, NULL, &size, &fmt);
		if (s != noErr) { fail(info, "tap format", s); ta_stop(st); return NULL; }
		info->rate = fmt.mSampleRate;
		info->channels = (int)fmt.mChannelsPerFrame;

		// The aggregate device needs a clock: the default output device.
		AudioObjectID out = 0;
		size = sizeof out;
		addr = (AudioObjectPropertyAddress){kAudioHardwarePropertyDefaultSystemOutputDevice, kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain};
		s = AudioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, NULL, &size, &out);
		if (s != noErr) { fail(info, "default output", s); ta_stop(st); return NULL; }
		CFStringRef uid = NULL;
		size = sizeof uid;
		addr = (AudioObjectPropertyAddress){kAudioDevicePropertyDeviceUID, kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain};
		s = AudioObjectGetPropertyData(out, &addr, 0, NULL, &size, &uid);
		if (s != noErr) { fail(info, "output uid", s); ta_stop(st); return NULL; }

		NSDictionary *desc = @{
			@kAudioAggregateDeviceNameKey: @"transflowy-tap",
			@kAudioAggregateDeviceUIDKey: [[NSUUID UUID] UUIDString],
			@kAudioAggregateDeviceMainSubDeviceKey: (__bridge NSString *)uid,
			@kAudioAggregateDeviceIsPrivateKey: @YES,
			@kAudioAggregateDeviceIsStackedKey: @NO,
			@kAudioAggregateDeviceTapAutoStartKey: @YES,
			@kAudioAggregateDeviceSubDeviceListKey: @[@{@kAudioSubDeviceUIDKey: (__bridge NSString *)uid}],
			@kAudioAggregateDeviceTapListKey: @[@{@kAudioSubTapDriftCompensationKey: @YES, @kAudioSubTapUIDKey: [d.UUID UUIDString]}],
		};
		CFRelease(uid);
		s = AudioHardwareCreateAggregateDevice((__bridge CFDictionaryRef)desc, &st->agg);
		if (s != noErr) { fail(info, "create aggregate device", s); st->agg = 0; ta_stop(st); return NULL; }

		s = AudioDeviceCreateIOProcID(st->agg, ioproc, st, &st->proc);
		if (s != noErr) { fail(info, "create io proc", s); st->proc = NULL; ta_stop(st); return NULL; }
		s = AudioDeviceStart(st->agg, st->proc);
		if (s != noErr) { fail(info, "start", s); ta_stop(st); return NULL; }
		return st;
	}
}
