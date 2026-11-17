// Package p8png decodes PICO-8 .p8.png cartridges into their P8SCII source
// code. It is read-only and does not execute or write carts.
package p8png

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
)

const (
	// CartWidth and CartHeight are the standard PICO-8 cart image dimensions.
	CartWidth  = 160
	CartHeight = 205
	// MaxFileSize caps the PNG input accepted by the decoder.
	MaxFileSize = 4 << 20

	romSize  = 0x4300
	codeSize = 0x3d00
	cartSize = 0x8000
)

var (
	errPNGSize    = errors.New("p8png: PNG must be 160x205")
	errPayload    = errors.New("p8png: truncated cartridge payload")
	errCodeSize   = errors.New("p8png: decoded code exceeds the code area")
	errCompressed = errors.New("p8png: invalid compressed code")
)

// Result is the decoded cartridge.
type Result struct {
	// Code is the decoded P8SCII Lua source as Unicode text.
	Code string
	// Text is a minimal text .p8 cart containing the decoded code.
	Text string
	// Version is the cartridge version reported by the text wrapper.
	Version int
}

// DecodeFile reads and decodes a .p8.png cartridge.
func DecodeFile(path string) (Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Result{}, err
	}
	if info.Size() > MaxFileSize {
		return Result{}, fmt.Errorf("p8png: %s is larger than %d bytes", path, MaxFileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	return Decode(data)
}

// Decode decodes PNG cartridge bytes.
func Decode(data []byte) (Result, error) {
	if len(data) > MaxFileSize {
		return Result{}, fmt.Errorf("p8png: input is larger than %d bytes", MaxFileSize)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Result{}, fmt.Errorf("p8png: decode PNG header: %w", err)
	}
	if config.Width != CartWidth || config.Height != CartHeight {
		return Result{}, errPNGSize
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{}, fmt.Errorf("p8png: decode PNG: %w", err)
	}
	bounds := img.Bounds()

	payload := make([]byte, 0, CartWidth*CartHeight)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			payload = append(payload,
				(c.B&3)|((c.G&3)<<2)|((c.R&3)<<4)|((c.A&3)<<6))
		}
	}
	if len(payload) < cartSize {
		return Result{}, errPayload
	}
	codeBytes := payload[romSize:cartSize]
	code, err := decodeCode(codeBytes)
	if err != nil {
		return Result{}, err
	}
	codeText := decodeP8SCII(code)
	text := "pico-8 cartridge // pico-8.com\nversion 42\n__lua__\n" + codeText
	return Result{Code: codeText, Text: text, Version: 42}, nil
}

func decodeCode(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, errPayload
	}
	switch {
	case bytes.Equal(data[:4], []byte{0, 'p', 'x', 'a'}):
		return decodeNew(data)
	case bytes.Equal(data[:4], []byte{':', 'c', ':', 0}):
		return decodeOld(data)
	default:
		// Uncompressed source is NUL-terminated or fills the code area.
		end := bytes.IndexByte(data, 0)
		if end < 0 {
			end = len(data)
		}
		if end > codeSize {
			return nil, errCodeSize
		}
		return append([]byte(nil), data[:end]...), nil
	}
}

func decodeNew(data []byte) ([]byte, error) {
	if len(data) < 8 {
		return nil, errPayload
	}
	uncSize := int(binary.BigEndian.Uint16(data[4:6]))
	comSize := int(binary.BigEndian.Uint16(data[6:8]))
	if comSize < 8 || comSize > len(data) || uncSize < 0 || uncSize > 0xffff {
		return nil, errCompressed
	}
	br := bitReader{data: data[8:comSize]}
	out := make([]byte, 0, uncSize)
	mtf := make([]byte, 256)
	for i := range mtf {
		mtf[i] = byte(i)
	}
	for len(out) < uncSize {
		if br.bit() == 1 {
			extra := 0
			for br.bit() == 1 {
				extra++
				if extra > 12 {
					return nil, errCompressed
				}
			}
			index, err := br.bits(4 + extra)
			if err != nil {
				return nil, errCompressed
			}
			index += ((1 << extra) - 1) << 4
			if index < 0 || index >= len(mtf) {
				return nil, errCompressed
			}
			ch := mtf[index]
			out = append(out, ch)
			copy(mtf[1:index+1], mtf[:index])
			mtf[0] = ch
			continue
		}

		offlen := 15
		if br.bit() == 1 {
			if br.bit() == 1 {
				offlen = 5
			} else {
				offlen = 10
			}
		}
		offset, err := br.bits(offlen)
		if err != nil {
			return nil, errCompressed
		}
		offset++
		if offset == 1 && offlen == 10 {
			for {
				ch, err := br.bits(8)
				if err != nil {
					return nil, errCompressed
				}
				if ch == 0 {
					break
				}
				out = append(out, byte(ch))
				if len(out) > uncSize {
					return nil, errCompressed
				}
			}
			continue
		}
		count := 3
		for {
			part, err := br.bits(3)
			if err != nil {
				return nil, errCompressed
			}
			count += part
			if part != 7 {
				break
			}
			if count > uncSize+7 {
				return nil, errCompressed
			}
		}
		if offset <= 0 || offset > len(out) || count > uncSize-len(out) {
			return nil, errCompressed
		}
		for i := 0; i < count; i++ {
			out = append(out, out[len(out)-offset])
		}
	}
	if len(out) != uncSize {
		return nil, errCompressed
	}
	return out, nil
}

func decodeOld(data []byte) ([]byte, error) {
	if len(data) < 8 {
		return nil, errPayload
	}
	uncSize := int(binary.BigEndian.Uint16(data[4:6]))
	if binary.BigEndian.Uint16(data[6:8]) != 0 {
		return nil, errCompressed
	}
	r := bytes.NewReader(data[8:])
	out := make([]byte, 0, uncSize)
	for {
		ch, err := r.ReadByte()
		if err != nil {
			return nil, errCompressed
		}
		if ch == 0 {
			ch2, err := r.ReadByte()
			if err != nil {
				return nil, errCompressed
			}
			if ch2 == 0 {
				break
			}
			out = append(out, ch2)
			continue
		}
		if ch <= 0x3b {
			out = append(out, oldCodeTable[ch])
			continue
		}
		ch2, err := r.ReadByte()
		if err != nil {
			return nil, errCompressed
		}
		count := int(ch2>>4) + 2
		offset := (int(ch-0x3c) << 4) + int(ch2&0x0f)
		if offset <= 0 || offset > len(out) || count <= 0 || count > uncSize-len(out) {
			return nil, errCompressed
		}
		for i := 0; i < count; i++ {
			out = append(out, out[len(out)-offset])
		}
		if len(out) > uncSize {
			return nil, errCompressed
		}
	}
	if len(out) != uncSize && len(out) != uncSize-1 {
		return nil, errCompressed
	}
	return out, nil
}

type bitReader struct {
	data      []byte
	pos       int
	bitOffset uint
}

func (r *bitReader) bit() int {
	if r.pos >= len(r.data) {
		return 0
	}
	value := int((r.data[r.pos] >> r.bitOffset) & 1)
	r.bitOffset++
	if r.bitOffset == 8 {
		r.bitOffset = 0
		r.pos++
	}
	return value
}

func (r *bitReader) bits(n int) (int, error) {
	value := 0
	for i := 0; i < n; i++ {
		if r.pos >= len(r.data) {
			return 0, io.ErrUnexpectedEOF
		}
		value |= r.bit() << i
	}
	return value, nil
}
