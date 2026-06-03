package media

import "github.com/liyuhui/micro-uac/internal/domain"

func PayloadTypeForCodec(codec domain.Codec) uint8 {
	switch codec.Canonical() {
	case domain.CodecPCMA:
		return 8
	default:
		return 0
	}
}
