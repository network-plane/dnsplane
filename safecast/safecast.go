// Package safecast holds explicit numeric conversions for metrics and wire formats
// so callers avoid unchecked int→uint casts flagged by static analysis.
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
package safecast

import (
	"math"
	"time"
)

// DurationToUint64 converts a wall-clock duration to uint64 nanoseconds for metrics.
// Non-positive values (clock skew) become 0.
func DurationToUint64(d time.Duration) uint64 {
	if d <= 0 {
		return 0
	}
	return uint64(d) // #nosec G115 -- d is strictly positive after check
}

// NonNegativeInt64ToUint64 converts v when v >= 0; otherwise returns 0.
func NonNegativeInt64ToUint64(v int64) uint64 {
	if v <= 0 {
		return 0
	}
	return uint64(v) // #nosec G115 -- v is strictly positive after check
}

// NonNegativeIntToUint64 converts v when v >= 0; otherwise returns 0.
func NonNegativeIntToUint64(v int) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v) // #nosec G115 -- v is non-negative after check
}

// IntToUint32Clamp converts v to uint32, saturating at math.MaxUint32 when v is larger.
func IntToUint32Clamp(v int) uint32 {
	if v <= 0 {
		return 0
	}
	if v > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(v) // #nosec G115 -- v clamped to uint32 range
}

// UnixSecondsToDNSUint32 converts Unix seconds to a 32-bit field as used in DNS RRSIG windows.
func UnixSecondsToDNSUint32(sec int64) uint32 {
	if sec <= 0 {
		return 0
	}
	if sec > int64(math.MaxUint32) {
		return math.MaxUint32
	}
	return uint32(sec) // #nosec G115 -- clamped to uint32 range
}
