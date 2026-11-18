// Package p8compress implements PICO-8's pxa code compression.
//
// The compressor is a Go port of Lexaloffle's MIT-licensed
// pxa_compress_snippets.c (PICO-8 0.2.4c) and is used only to estimate the
// compressed code size that PICO-8 would store for a cart. It does not write
// cartridges.
//
// Portions derived from pxa_compress_snippets.c:
//
// # Copyright (c) 2020-22 Lexaloffle Games LLP
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package p8compress

const (
	// HeaderSize is the size of the pxa header, included in the reported size.
	HeaderSize = 8
	// MaxInput is the largest code PICO-8 can store (16-bit length field).
	MaxInput = 0xffff
	// DefaultWorkLimit bounds match-candidate work so pathological input cannot
	// stall analysis. Normal carts use far less. Exceeding the limit makes the
	// result unavailable instead of producing a wrong size.
	DefaultWorkLimit = 16 << 20
)

const (
	minBlockLen       = 3
	blockLenChainBits = 3
	blockDistBits     = 5
	tinyLiteralBits   = 4
	maxHistoryLen     = 32767
	hashMax           = 4096
)

// Compress returns the bytes PICO-8 would store for code, using the default
// work limit. The result is either a pxa stream (including its 8-byte header)
// or the input itself when compression would not help. ok is false when the
// input is too large or the work limit was exceeded.
func Compress(code []byte) (out []byte, ok bool) {
	return CompressLimit(code, DefaultWorkLimit)
}

// CompressLimit is Compress with an explicit match work limit.
func CompressLimit(code []byte, workLimit int) (out []byte, ok bool) {
	if len(code) == 0 {
		return []byte{}, true
	}
	if len(code) > MaxInput {
		return nil, false
	}
	if workLimit <= 0 {
		workLimit = DefaultWorkLimit
	}

	c := compressor{
		in:      code,
		work:    workLimit,
		matches: make([]matchResult, len(code)),
	}
	c.buildHashLists()
	c.initMTF()

	capacity := len(code)*2 + HeaderSize + 64
	c.out = bitWriter{buf: make([]byte, capacity), mask: 1}

	// pxa header: \0pxa, uncompressed length, compressed length (filled in
	// after the stream is complete).
	c.writeVal(0, 8)
	c.writeVal('p', 8)
	c.writeVal('x', 8)
	c.writeVal('a', 8)
	c.writeVal(len(code)/256, 8)
	c.writeVal(len(code)%256, 8)
	c.writeVal(0, 8)
	c.writeVal(0, 8)

	rawPosDest := c.out.pos
	rawPosSrc := 0
	rawPosSrc0 := 0
	rawHeaderWritePos := c.out.writePos()
	rawBlockWritePos := rawHeaderWritePos
	var literalBackup, literalPosBackup [256]byte
	backupLiterals := func() {
		literalBackup = c.literal
		literalPosBackup = c.literalPos
	}
	restoreLiterals := func() {
		c.literal = literalBackup
		c.literalPos = literalPosBackup
	}
	backupLiterals()
	storedLastSegmentAsRaw := false

	for pos := 0; pos < len(code); {
		blockOffset, blockLen, blockScore := c.findRepeatableBlock(pos)
		if c.failed {
			return nil, false
		}

		ch := code[pos]
		lpos := int(c.literalPos[ch])
		catBits := tinyLiteralBits
		catMaxVal := 1 << catBits
		for lpos >= catMaxVal {
			catBits++
			catMaxVal += 1 << catBits
		}
		literalScore := 256 / (2 + (catBits - tinyLiteralBits) + catBits)

		if blockLen >= minBlockLen && blockScore > literalScore && blockScore < 128 {
			// Before committing to a modest match, check whether a better one
			// starts in the next two bytes. This is part of PICO-8's heuristic
			// and materially changes the output.
			for lookahead := 1; lookahead < 3; lookahead++ {
				_, _, score2 := c.findRepeatableBlock(pos + lookahead)
				if score2 > blockScore*6/5 {
					blockScore = 0
					break
				}
			}
		}

		if blockLen >= minBlockLen && blockScore > literalScore && !c.failed {
			c.writeBit(0)
			c.writeNum(blockOffset - 1)
			c.writeChain(blockLen-minBlockLen, blockLenChainBits, 100000)
			pos += blockLen
		} else {
			c.writeBit(1)
			val := lpos
			catBits = tinyLiteralBits
			catMaxVal = 1 << catBits
			for lpos >= catMaxVal {
				val -= 1 << catBits
				catBits++
				catMaxVal += 1 << catBits
			}
			c.writeChain(catBits-tinyLiteralBits, 1, 16)
			c.writeVal(val, catBits)

			for i := lpos; i > 0; i-- {
				c.literal[i] = c.literal[i-1]
				c.literalPos[c.literal[i]] = byte(i)
			}
			c.literal[0] = ch
			c.literalPos[ch] = 0

			pos++
		}

		if c.out.pos-rawPosDest >= 32 || pos == len(code) {
			compressedSize := c.out.pos - rawPosDest
			rawSize := pos - rawPosSrc
			margin := 0
			if rawPosSrc0 == rawPosSrc {
				margin = 3 // header + null terminator for the first section
			}

			if compressedSize > rawSize+margin {
				if !storedLastSegmentAsRaw {
					rawHeaderWritePos = rawBlockWritePos
					c.out.setWritePos(rawHeaderWritePos)
					c.writeBit(0)
					c.writeBit(1)
					c.writeBit(0)
					c.writeVal(0, 10)
				} else {
					c.out.setWritePos(rawBlockWritePos)
					c.out.pos-- // overwrite the previous null terminator
				}
				for k := 0; k < rawSize; k++ {
					c.writeVal(int(code[rawPosSrc+k]), 8)
				}
				c.writeVal(0, 8)
				storedLastSegmentAsRaw = true
				restoreLiterals()
			} else {
				storedLastSegmentAsRaw = false
				rawPosSrc0 = pos
				backupLiterals()
			}

			rawPosDest = c.out.pos
			rawPosSrc = pos
			rawBlockWritePos = c.out.writePos()
		}

		if c.failed {
			return nil, false
		}
	}

	for c.out.mask != 1 {
		c.writeBit(0)
	}
	bytesWritten := c.out.pos
	c.out.buf[6] = byte(bytesWritten / 256)
	c.out.buf[7] = byte(bytesWritten % 256)

	if c.failed {
		return nil, false
	}
	if bytesWritten > len(code) {
		return append([]byte(nil), code...), true
	}
	return c.out.buf[:bytesWritten], true
}

// Size returns the stored size of code. ok is false when the size is
// unavailable (oversized or too expensive to compute).
func Size(code []byte) (int, bool) {
	out, ok := Compress(code)
	if !ok {
		return 0, false
	}
	return len(out), true
}

type compressor struct {
	in         []byte
	out        bitWriter
	literal    [256]byte
	literalPos [256]byte
	hashStart  []int
	hashPos    []int
	matches    []matchResult
	work       int
	failed     bool
}

type matchResult struct {
	offset int
	length int
	score  int
	valid  bool
}

func (c *compressor) initMTF() {
	for i := 0; i < 256; i++ {
		c.literal[i] = byte(i)
		c.literalPos[i] = byte(i)
	}
}

func miniHash(data []byte, i int) int {
	return (int(data[i])*7 + int(data[i+1])*1503 + int(data[i+2])*51717) & (hashMax - 1)
}

// buildHashLists indexes every 3-byte sequence by hash. Positions in each
// bucket remain in ascending order, matching PICO-8's iteration order.
func (c *compressor) buildHashLists() {
	var counts [hashMax + 1]int
	for i := 0; i+2 < len(c.in); i++ {
		counts[miniHash(c.in, i)+1]++
	}
	for i := 1; i <= hashMax; i++ {
		counts[i] += counts[i-1]
	}
	c.hashStart = counts[:]
	c.hashPos = make([]int, counts[hashMax])
	var cursor [hashMax]int
	copy(cursor[:], c.hashStart[:hashMax])
	for i := 0; i+2 < len(c.in); i++ {
		hash := miniHash(c.in, i)
		c.hashPos[cursor[hash]] = i
		cursor[hash]++
	}
}

// findRepeatableBlock returns the best historical match at pos using PICO-8's
// cost heuristic. It mirrors pxa_find_repeatable_block.
func (c *compressor) findRepeatableBlock(pos int) (blockOffset, blockLen, scoreOut int) {
	if pos < len(c.matches) && c.matches[pos].valid {
		cached := c.matches[pos]
		return cached.offset, cached.length, cached.score
	}
	offset, length, score := c.findRepeatableBlockUncached(pos)
	if !c.failed && pos >= 0 && pos < len(c.matches) {
		c.matches[pos] = matchResult{offset, length, score, true}
	}
	return offset, length, score
}

func (c *compressor) findRepeatableBlockUncached(pos int) (blockOffset, blockLen, scoreOut int) {
	maxLen := len(c.in) - pos
	if maxLen < minBlockLen {
		return 0, 0, -1
	}
	hash := miniHash(c.in, pos)
	bucketStart, bucketEnd := c.hashStart[hash], c.hashStart[hash+1]
	work := c.work

	bestLen := 0
	bestPos0 := -100000
	bestScore := -1
	maxScoreNumerator := maxLen * 256
	for i := bucketStart; i < bucketEnd; i++ {
		pos0 := c.hashPos[i]
		if pos0 >= pos {
			break
		}
		if pos0 < pos-maxHistoryLen {
			continue
		}

		work--
		if work < 0 {
			c.work = work
			c.failed = true
			return 0, 0, -1
		}

		dist := pos - pos0
		bitCost := 10 // 6 distance bits + 4 for the block marker and length
		if dist > 31 {
			bitCost = 16
		}
		if dist > 1023 {
			bitCost = 21
		}

		// Even a full-length match cannot strictly beat the current best.
		// Ties keep the earlier candidate, matching PICO-8's update rule.
		if bestScore >= 0 && maxScoreNumerator <= bestScore*bitCost {
			continue
		}

		match := 0
		period := pos - pos0
		limit := maxLen
		if period < limit {
			limit = period
		}
		before := c.in[pos0:]
		after := c.in[pos:]
		for match < limit && before[match] == after[match] {
			match++
			work--
		}
		if match == period {
			j := 0
			for match < maxLen {
				if before[j] != after[match] {
					break
				}
				match++
				j++
				if j == period {
					j = 0
				}
				work--
			}
		}
		if work < 0 {
			c.work = work
			c.failed = true
			return 0, 0, -1
		}

		score := match * 256 / bitCost
		if score > bestScore {
			bestScore = score
			bestPos0 = pos0
			bestLen = match
		}
	}
	c.work = work

	blockOffset = 0
	if bestPos0 >= 0 {
		blockOffset = pos - bestPos0
	}
	return blockOffset, bestLen, bestScore
}

func (c *compressor) writeBit(value int) {
	c.out.writeBit(value)
	if c.out.failed {
		c.failed = true
	}
}

func (c *compressor) writeVal(value, bits int) {
	c.out.writeVal(value, bits)
	if c.out.failed {
		c.failed = true
	}
}

func (c *compressor) writeChain(value, linkBits, maxBits int) {
	c.out.writeChain(value, linkBits, maxBits)
	if c.out.failed {
		c.failed = true
	}
}

// writeNum encodes a block distance with PICO-8's 5/10/15-bit selector.
func (c *compressor) writeNum(value int) {
	bits := blockDistBits
	for (1 << bits) <= value {
		bits += blockDistBits
	}
	c.writeChain(3-(bits/blockDistBits), 1, 2)
	c.writeVal(value, bits)
}

type bitWriter struct {
	buf    []byte
	pos    int
	mask   uint8
	failed bool
}

func (w *bitWriter) writeBit(value int) {
	if w.failed {
		return
	}
	if w.pos >= len(w.buf) {
		w.failed = true
		return
	}
	if value != 0 {
		w.buf[w.pos] |= w.mask
	} else {
		w.buf[w.pos] &^= w.mask
	}
	if w.mask == 0x80 {
		w.mask = 1
		w.pos++
	} else {
		w.mask <<= 1
	}
}

func (w *bitWriter) writeVal(value, bits int) {
	for i := 0; i < bits; i++ {
		w.writeBit(value & (1 << i))
	}
}

func (w *bitWriter) writeChain(value, linkBits, maxBits int) {
	maxLinkVal := (1 << linkBits) - 1
	bitsWritten := 0
	vv := maxLinkVal
	for vv == maxLinkVal {
		vv = min(value, maxLinkVal)
		w.writeVal(vv, linkBits)
		bitsWritten += linkBits
		value -= vv
		if bitsWritten >= maxBits {
			return
		}
	}
}

func (w *bitWriter) writePos() int {
	return (w.pos << 8) | int(w.mask)
}

func (w *bitWriter) setWritePos(pos int) {
	w.pos = pos >> 8
	w.mask = uint8(pos & 0xff)
}
