package source

// FileID identifies a file in a cart's file table.
type FileID int

// Span is a byte range in one original source file.
type Span struct {
	FileID FileID
	Start  int
	End    int
}

// Len returns the number of bytes in the span.
func (s Span) Len() int {
	if s.End < s.Start {
		return 0
	}
	return s.End - s.Start
}

// IsEmpty reports whether the span contains no bytes.
func (s Span) IsEmpty() bool { return s.End <= s.Start }

// Contains reports whether offset lies inside the span. An empty span
// contains only its start offset.
func (s Span) Contains(offset int) bool {
	if s.IsEmpty() {
		return offset == s.Start
	}
	return offset >= s.Start && offset < s.End
}

// Merge returns the smallest span covering both inputs. Spans in different
// files cannot be merged; the first span is returned in that case.
func (s Span) Merge(other Span) Span {
	if s.FileID != other.FileID {
		return s
	}
	if other.Start < s.Start {
		s.Start = other.Start
	}
	if other.End > s.End {
		s.End = other.End
	}
	return s
}
