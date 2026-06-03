package sdp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/liyuhui/micro-uac/internal/domain"
	"github.com/liyuhui/micro-uac/internal/media"
)

type Offer struct {
	Body        string
	ListenIP    string
	ListenPort  int
	PayloadType uint8
	Codec       domain.Codec
}

func BuildOffer(ip string, port int, codec domain.Codec) Offer {
	codec = codec.Canonical()
	payload := media.PayloadTypeForCodec(codec)
	name := "PCMU"
	if codec == domain.CodecPCMA {
		name = "PCMA"
	}
	body := fmt.Sprintf("v=0\r\n"+
		"o=- 0 0 IN IP4 %s\r\n"+
		"s=micro-uac\r\n"+
		"c=IN IP4 %s\r\n"+
		"t=0 0\r\n"+
		"m=audio %d RTP/AVP %d\r\n"+
		"a=rtpmap:%d %s/8000\r\n"+
		"a=sendonly\r\n", ip, ip, port, payload, payload, name)
	return Offer{
		Body:        body,
		ListenIP:    ip,
		ListenPort:  port,
		PayloadType: payload,
		Codec:       codec,
	}
}

func ParseAnswer(body string, preferred domain.Codec) (domain.RemoteMedia, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var (
		host     string
		port     int
		payloads []int
		rtpmap   = map[int]string{}
	)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "c="):
			parts := strings.Fields(strings.TrimPrefix(line, "c="))
			if len(parts) >= 3 {
				host = parts[2]
			}
		case strings.HasPrefix(line, "m=audio "):
			parts := strings.Fields(line)
			if len(parts) < 4 {
				return domain.RemoteMedia{}, fmt.Errorf("invalid m=audio line")
			}
			parsedPort, err := strconv.Atoi(parts[1])
			if err != nil {
				return domain.RemoteMedia{}, fmt.Errorf("parse media port: %w", err)
			}
			port = parsedPort
			for _, p := range parts[3:] {
				v, err := strconv.Atoi(p)
				if err == nil {
					payloads = append(payloads, v)
				}
			}
		case strings.HasPrefix(line, "a=rtpmap:"):
			payload, codecName, ok := parseRTPMap(line)
			if ok {
				rtpmap[payload] = codecName
			}
		}
	}

	if host == "" || port == 0 {
		return domain.RemoteMedia{}, fmt.Errorf("incomplete remote media in sdp answer")
	}

	preferred = preferred.Canonical()
	tryOrder := []domain.Codec{preferred}
	if preferred == domain.CodecPCMU {
		tryOrder = append(tryOrder, domain.CodecPCMA)
	} else {
		tryOrder = append(tryOrder, domain.CodecPCMU)
	}

	for _, codec := range tryOrder {
		name := strings.ToUpper(string(codec))
		for _, payload := range payloads {
			if payload == 0 && codec == domain.CodecPCMU {
				return domain.RemoteMedia{Host: host, Port: port, PayloadType: 0, Codec: codec}, nil
			}
			if payload == 8 && codec == domain.CodecPCMA {
				return domain.RemoteMedia{Host: host, Port: port, PayloadType: 8, Codec: codec}, nil
			}
			if strings.EqualFold(rtpmap[payload], name+"/8000") || strings.EqualFold(rtpmap[payload], name) {
				return domain.RemoteMedia{Host: host, Port: port, PayloadType: uint8(payload), Codec: codec}, nil
			}
		}
	}

	return domain.RemoteMedia{}, fmt.Errorf("no supported codec found in sdp answer")
}

func parseRTPMap(line string) (int, string, bool) {
	value := strings.TrimPrefix(line, "a=rtpmap:")
	parts := strings.Fields(value)
	if len(parts) != 2 {
		return 0, "", false
	}
	payload, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	return payload, parts[1], true
}
