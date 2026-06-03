package media

// Linear PCM to G.711 conversion adapted from ITU-T reference equations.

const (
	muBias = 0x84
	muClip = 32635
)

var segEnd = [8]int16{0xFF, 0x1FF, 0x3FF, 0x7FF, 0xFFF, 0x1FFF, 0x3FFF, 0x7FFF}

func EncodePCMU(pcm []int16) []byte {
	out := make([]byte, len(pcm))
	for i, sample := range pcm {
		out[i] = linearToMuLaw(sample)
	}
	return out
}

func EncodePCMA(pcm []int16) []byte {
	out := make([]byte, len(pcm))
	for i, sample := range pcm {
		out[i] = linearToALaw(sample)
	}
	return out
}

func linearToMuLaw(sample int16) byte {
	sign := byte(0)
	s := int(sample)
	if s < 0 {
		sign = 0x80
		s = -s
		if s > 32767 {
			s = 32767
		}
	}
	if s > muClip {
		s = muClip
	}
	s += muBias
	segment := findSegment(int16(s))
	mantissa := byte((s >> (segment + 3)) & 0x0F)
	return ^(sign | byte(segment<<4) | mantissa)
}

func linearToALaw(sample int16) byte {
	sign := byte(0x80)
	s := int(sample)
	if s >= 0 {
		sign = 0x00
	} else {
		s = -s - 1
	}

	segment := findSegment(int16(s))
	var aval byte
	if segment >= 8 {
		aval = 0x7F
	} else {
		aval = byte(segment << 4)
		if segment < 2 {
			aval |= byte((s >> 4) & 0x0F)
		} else {
			aval |= byte((s >> (segment + 3)) & 0x0F)
		}
	}
	return aval ^ (sign ^ 0x55)
}

func findSegment(sample int16) int {
	for i, end := range segEnd {
		if sample <= end {
			return i
		}
	}
	return 8
}
