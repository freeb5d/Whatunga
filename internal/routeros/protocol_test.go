package routeros

import (
	"bufio"
	"bytes"
	"testing"
)

func TestWriteReadLength_RoundTrip(t *testing.T) {
	lengths := []int{0, 1, 0x7F, 0x80, 0x3FFF, 0x4000, 0x1FFFFF, 0x200000, 0xFFFFFF, 0x10000000}

	for _, length := range lengths {
		var buf bytes.Buffer
		if err := writeLength(&buf, length); err != nil {
			t.Fatalf("writeLength(%d): %v", length, err)
		}

		got, err := readLength(bufio.NewReader(&buf))
		if err != nil {
			t.Fatalf("readLength after writing %d: %v", length, err)
		}
		if got != length {
			t.Errorf("round-trip mismatch: wrote %d, read back %d", length, got)
		}
	}
}

func TestWriteReadSentence_RoundTrip(t *testing.T) {
	words := []string{"/system/resource/print", "=interface=ether1", "=stats="}

	var buf bytes.Buffer
	if err := writeSentence(&buf, words); err != nil {
		t.Fatalf("writeSentence: %v", err)
	}

	got, err := readSentence(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("readSentence: %v", err)
	}

	if len(got) != len(words) {
		t.Fatalf("got %d words, want %d: %v", len(got), len(words), got)
	}
	for i := range words {
		if got[i] != words[i] {
			t.Errorf("word %d: got %q, want %q", i, got[i], words[i])
		}
	}
}

func TestParseAttributeWords(t *testing.T) {
	attrs := parseAttributeWords([]string{"=name=ether1", "=running=true", "not-an-attr"})

	if attrs["name"] != "ether1" {
		t.Errorf("name = %q, want %q", attrs["name"], "ether1")
	}
	if attrs["running"] != "true" {
		t.Errorf("running = %q, want %q", attrs["running"], "true")
	}
	if _, ok := attrs["not-an-attr"]; ok {
		t.Errorf("malformed word should not have been parsed as an attribute")
	}
}
