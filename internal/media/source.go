package media

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liyuhui/micro-uac/internal/domain"
)

type Source struct {
	codec        domain.Codec
	frameSamples int
	data         []int16
	offset       int
}

func NewSource(path string, codec domain.Codec, frameMS int) (*Source, error) {
	if frameMS <= 0 {
		return nil, errors.New("frame duration must be greater than 0")
	}
	if err := codec.Canonical().Validate(); err != nil {
		return nil, err
	}

	pcm, err := readPCMFile(path)
	if err != nil {
		return nil, err
	}

	frameSamples := 8000 * frameMS / 1000
	if frameSamples <= 0 {
		return nil, errors.New("frame duration produces zero samples")
	}

	return &Source{
		codec:        codec.Canonical(),
		frameSamples: frameSamples,
		data:         pcm,
	}, nil
}

func (s *Source) FrameSamples() int {
	return s.frameSamples
}

func (s *Source) NextFrame() ([]byte, bool, error) {
	if s.offset >= len(s.data) {
		return nil, false, nil
	}

	end := s.offset + s.frameSamples
	frame := make([]int16, s.frameSamples)
	if end <= len(s.data) {
		copy(frame, s.data[s.offset:end])
		s.offset = end
	} else {
		copy(frame, s.data[s.offset:])
		s.offset = len(s.data)
	}

	switch s.codec {
	case domain.CodecPCMA:
		return EncodePCMA(frame), true, nil
	default:
		return EncodePCMU(frame), true, nil
	}
}

func readPCMFile(path string) ([]int16, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read audio file: %w", err)
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav":
		return parseWAVPCM(raw)
	case ".pcm":
		return bytesToPCM(raw)
	default:
		return nil, errors.New("unsupported audio file extension")
	}
}

func bytesToPCM(raw []byte) ([]int16, error) {
	if len(raw)%2 != 0 {
		return nil, errors.New("pcm byte length must be even")
	}
	pcm := make([]int16, len(raw)/2)
	for i := 0; i < len(pcm); i++ {
		pcm[i] = int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
	}
	return pcm, nil
}

func parseWAVPCM(raw []byte) ([]int16, error) {
	r := newChunkReader(raw)

	if id, err := r.readString(4); err != nil || id != "RIFF" {
		return nil, errors.New("invalid wav riff header")
	}
	if _, err := r.readUint32(); err != nil {
		return nil, err
	}
	if id, err := r.readString(4); err != nil || id != "WAVE" {
		return nil, errors.New("invalid wav wave header")
	}

	var (
		formatFound bool
		dataChunk   []byte
	)

	for r.remaining() >= 8 {
		chunkID, _ := r.readString(4)
		size, _ := r.readUint32()
		chunkData, err := r.readBytes(int(size))
		if err != nil {
			return nil, err
		}
		if size%2 == 1 {
			if _, err := r.readBytes(1); err != nil {
				return nil, err
			}
		}

		switch chunkID {
		case "fmt ":
			if len(chunkData) < 16 {
				return nil, errors.New("invalid wav fmt chunk")
			}
			audioFormat := binary.LittleEndian.Uint16(chunkData[0:2])
			channels := binary.LittleEndian.Uint16(chunkData[2:4])
			sampleRate := binary.LittleEndian.Uint32(chunkData[4:8])
			bitsPerSample := binary.LittleEndian.Uint16(chunkData[14:16])
			if audioFormat != 1 || channels != 1 || sampleRate != 8000 || bitsPerSample != 16 {
				return nil, errors.New("wav must be PCM 16-bit mono 8kHz")
			}
			formatFound = true
		case "data":
			dataChunk = chunkData
		}
	}

	if !formatFound {
		return nil, errors.New("wav fmt chunk missing")
	}
	if len(dataChunk) == 0 {
		return nil, errors.New("wav data chunk missing")
	}
	return bytesToPCM(dataChunk)
}

type chunkReader struct {
	raw []byte
	pos int
}

func newChunkReader(raw []byte) *chunkReader {
	return &chunkReader{raw: raw}
}

func (r *chunkReader) remaining() int {
	return len(r.raw) - r.pos
}

func (r *chunkReader) readBytes(n int) ([]byte, error) {
	if n < 0 || r.remaining() < n {
		return nil, io.ErrUnexpectedEOF
	}
	out := r.raw[r.pos : r.pos+n]
	r.pos += n
	return out, nil
}

func (r *chunkReader) readString(n int) (string, error) {
	b, err := r.readBytes(n)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *chunkReader) readUint32() (uint32, error) {
	b, err := r.readBytes(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}
