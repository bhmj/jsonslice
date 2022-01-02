package jsonslice

// readQuotedKey reads quoted key. Allocates memory if necessasry.
// Consumes right bound.
// Returns key []byte -- sliced or allocated (without quotes), i after last quote
func readQuotedKey(path []byte, i int) ([]byte, int, error) {
	l := len(path)
	var err error
	var esc []byte
	var result []byte
	bound := path[i] // ' or "
	i++
	s := i
	copying := false

	for i < l && path[i] != bound {
		if path[i] == '\\' {
			// get escaped value
			before := i
			esc, i, err = readEscape(path, i)
			if err != nil {
				return nil, i, err
			}
			if !copying {
				copying = true                             // start copying
				result = append(result, path[s:before]...) // copy from the start
			}
			result = append(result, esc...)
			s = i // we want to bulk append the rest
		} else {
			i++
		}
	}
	if i == l { // no bound
		return nil, i, errUnexpectedStringEnd
	}
	if copying {
		result = append(result, path[s:i]...)
	} else {
		result = path[s:i]
	}
	return result, i + 1, nil
}

// readEscape reads escape sequence. path[i] must be '\' symbol.
// Supports \n, \r, \t, \0, \', \", \\, \x, \u.
// Returns UTF-8 rune as []byte, next character pointer or error.
func readEscape(path []byte, i int) ([]byte, int, error) {
	l := len(path)
	i++
	if i == l {
		return nil, i, errPathUnexpectedEnd
	}
	switch path[i] {
	case 'n':
		return []byte{'\x0A'}, i + 1, nil
	case 'r':
		return []byte{'\x0D'}, i + 1, nil
	case 't':
		return []byte{'\x09'}, i + 1, nil
	case '0':
		return []byte{'\x00'}, i + 1, nil
	case '\'':
		return []byte{'\''}, i + 1, nil
	case '"':
		return []byte{'"'}, i + 1, nil
	case '\\':
		return []byte{'\\'}, i + 1, nil
	case 'x':
		return readHexByte(path, i+1)
	case 'u':
		return readUnicodeSequence(4, path, i+1)
	case 'U':
		return readUnicodeSequence(8, path, i+1)
	}
	return nil, i, errPathUnknownEscape
}

func readHexByte(path []byte, i int) ([]byte, int, error) {
	value := uint32(0)
	var err error

	l := len(path)
	for num := 0; i < l && num < 2; i++ {
		value, err = readNextHexNum(value, path[i])
		if err != nil {
			return nil, i, err
		}
		num++
	}
	return []byte{byte(value)}, i, nil
}

func readNextHexNum(value uint32, ch byte) (uint32, error) {
	if ch >= '0' && ch <= '9' {
		value = (value << 4) + uint32(ch-'0')
	} else if ch >= 'A' && ch <= 'F' {
		value = (value << 4) + uint32(ch-'A'+10)
	} else if ch >= 'a' && ch <= 'f' {
		value = (value << 4) + uint32(ch-'a'+10)
	} else {
		return value, errPathUnknownEscape
	}
	return value, nil
}

func readUnicodeSequence(length int, path []byte, i int) ([]byte, int, error) {
	var err error
	codepoint := uint32(0)
	l := len(path)
	num := 0
	for ; i < l && num < length; i++ {
		codepoint, err = readNextHexNum(codepoint, path[i])
		if err != nil {
			return nil, i, err
		}
		num++
	}
	if i == l && num < length {
		return nil, i, errPathUnexpectedEnd
	}
	res, err := codepointToUTF8(codepoint)
	return res, i, err
}

func codepointToUTF8(codepoint uint32) ([]byte, error) {
	if codepoint <= 0x7F {
		return []byte{byte(codepoint)}, nil
	} else if codepoint <= 0x7FF {
		return []byte{
			0xC0 + byte((codepoint>>6)&0x3F),
			0x80 + byte(codepoint&0x3F),
		}, nil
	} else if codepoint <= 0xFFFF {
		return []byte{
			0xE0 + byte((codepoint>>12)&0x3F),
			0x80 + byte((codepoint>>6)&0x3F),
			0x80 + byte(codepoint&0x3F),
		}, nil
	} else if codepoint <= 0x1FFFFF {
		return []byte{
			0xF0 + byte((codepoint>>18)&0x3F),
			0x80 + byte((codepoint>>12)&0x3F),
			0x80 + byte((codepoint>>6)&0x3F),
			0x80 + byte(codepoint&0x3F),
		}, nil
	} else if codepoint <= 0x3FFFFFF {
		return []byte{
			0xF8 + byte((codepoint>>24)&0x3F),
			0x80 + byte((codepoint>>18)&0x3F),
			0x80 + byte((codepoint>>12)&0x3F),
			0x80 + byte((codepoint>>6)&0x3F),
			0x80 + byte(codepoint&0x3F),
		}, nil
	} else if codepoint <= 0x7FFFFFFF {
		return []byte{
			0xFC + byte((codepoint>>30)&0x3F),
			0x80 + byte((codepoint>>24)&0x3F),
			0x80 + byte((codepoint>>18)&0x3F),
			0x80 + byte((codepoint>>12)&0x3F),
			0x80 + byte((codepoint>>6)&0x3F),
			0x80 + byte(codepoint&0x3F),
		}, nil
	} else {
		return nil, errPathUnknownEscape
	}
}

func toInt(buf []byte) int {
	if len(buf) == 0 {
		return cEmpty
	}
	n := 0
	sign := 1
	for _, ch := range buf {
		if ch == '-' {
			sign = -1
			continue
		}
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		} else {
			return cNAN
		}
	}
	return n * sign
}

func readTerminatorBounded(path []byte, i int, terminators []byte) ([]byte, int, error) {
	l := len(path)
	s := i

	if path[i] == '-' { // CAUTION: allow '-' at the start (to support negative numbers)
		i++
	}

	for i < l {
		if bytein(path[i], terminators) {
			if path[i] == '*' && s-i == 0 {
				i++ // CAUTION: usually '*' is a terminator but not in '.*' so skip it
			}
			break
		}
		i++
	}

	return path[s:i], i, nil
}
