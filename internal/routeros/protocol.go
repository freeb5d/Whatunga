// Package routeros implements a minimal client for MikroTik's RouterOS API
// (the binary protocol exposed on TCP port 8728, or 8729 for API-SSL).
//
// This is a from-scratch implementation of the wire protocol described in
// MikroTik's own API documentation — no third-party client library is used.
// The protocol is deceptively simple once you see it: a "sentence" is a
// sequence of length-prefixed "words", terminated by a zero-length word.
//
// Word length encoding (variable-length, similar in spirit to UTF-8's
// continuation-byte scheme):
//
//	length < 0x80        -> 1 byte:  0LLLLLLL
//	length < 0x4000       -> 2 bytes: 01LLLLLL LLLLLLLL
//	length < 0x200000      -> 3 bytes: 011LLLLL LLLLLLLL LLLLLLLL
//	length < 0x10000000     -> 4 bytes: 0111LLLL LLLLLLLL LLLLLLLL LLLLLLLL
//	otherwise               -> 5 bytes: 11110000 + 4 raw length bytes
package routeros

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

// writeLength encodes a word length using RouterOS's variable-length scheme.
func writeLength(w io.Writer, length int) error {
	switch {
	case length < 0x80:
		_, err := w.Write([]byte{byte(length)})
		return err
	case length < 0x4000:
		length |= 0x8000
		return binary.Write(w, binary.BigEndian, uint16(length))
	case length < 0x200000:
		buf := []byte{
			byte(length>>16) | 0xC0,
			byte(length >> 8),
			byte(length),
		}
		_, err := w.Write(buf)
		return err
	case length < 0x10000000:
		buf := []byte{
			byte(length>>24) | 0xE0,
			byte(length >> 16),
			byte(length >> 8),
			byte(length),
		}
		_, err := w.Write(buf)
		return err
	default:
		buf := []byte{0xF0}
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(length))
		buf = append(buf, lenBuf...)
		_, err := w.Write(buf)
		return err
	}
}

// readLength decodes a word length prefix, returning the length in bytes.
func readLength(r *bufio.Reader) (int, error) {
	first, err := r.ReadByte()
	if err != nil {
		return 0, err
	}

	switch {
	case first&0x80 == 0x00:
		return int(first), nil
	case first&0xC0 == 0x80:
		b2, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		return int(first&^0xC0)<<8 | int(b2), nil
	case first&0xE0 == 0xC0:
		rest := make([]byte, 2)
		if _, err := io.ReadFull(r, rest); err != nil {
			return 0, err
		}
		return int(first&^0xE0)<<16 | int(rest[0])<<8 | int(rest[1]), nil
	case first&0xF0 == 0xE0:
		rest := make([]byte, 3)
		if _, err := io.ReadFull(r, rest); err != nil {
			return 0, err
		}
		return int(first&^0xF0)<<24 | int(rest[0])<<16 | int(rest[1])<<8 | int(rest[2]), nil
	case first == 0xF0:
		rest := make([]byte, 4)
		if _, err := io.ReadFull(r, rest); err != nil {
			return 0, err
		}
		return int(binary.BigEndian.Uint32(rest)), nil
	default:
		return 0, fmt.Errorf("routeros: invalid length prefix byte 0x%02x", first)
	}
}

// writeWord writes a single length-prefixed word.
func writeWord(w io.Writer, word string) error {
	if err := writeLength(w, len(word)); err != nil {
		return err
	}
	_, err := w.Write([]byte(word))
	return err
}

// readWord reads a single length-prefixed word. A zero-length word
// signals the end of the sentence, and is returned as ("", nil).
func readWord(r *bufio.Reader) (string, error) {
	length, err := readLength(r)
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// writeSentence writes a full sentence (a command plus its attribute
// words) followed by the terminating zero-length word.
func writeSentence(w io.Writer, words []string) error {
	for _, word := range words {
		if err := writeWord(w, word); err != nil {
			return fmt.Errorf("routeros: writing word %q: %w", word, err)
		}
	}
	return writeLength(w, 0)
}

// readSentence reads words until it hits the terminating zero-length word.
func readSentence(r *bufio.Reader) ([]string, error) {
	var words []string
	for {
		word, err := readWord(r)
		if err != nil {
			return nil, err
		}
		if word == "" {
			return words, nil
		}
		words = append(words, word)
	}
}
