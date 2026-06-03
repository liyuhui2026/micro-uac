package service

import (
	"fmt"
	"net"
)

func reserveUDPPort() (int, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port, nil
}

func hostFromListenAddr(addr string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("parse sip.listen_addr: %w", err)
	}
	if host == "" || host == "0.0.0.0" {
		return "127.0.0.1", nil
	}
	return host, nil
}
