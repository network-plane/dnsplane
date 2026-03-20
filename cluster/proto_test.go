// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only

package cluster

import (
	"bytes"
	"testing"
)

func TestWriteReadFrameRoundTrip(t *testing.T) {
	payload := []byte(`{"type":"ping"}`)
	var buf bytes.Buffer
	if err := WriteFrame(&buf, payload); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q want %q", got, payload)
	}
}

func TestReadFrameTooLarge(t *testing.T) {
	var hdr [4]byte
	// 64 MiB + 1 — exceeds MaxFrameBytes
	hdr[0] = 0x04
	hdr[1] = 0x00
	hdr[2] = 0x00
	hdr[3] = 0x01
	r := bytes.NewReader(hdr[:])
	_, err := ReadFrame(r)
	if err == nil {
		t.Fatal("expected error for oversized frame length")
	}
}
