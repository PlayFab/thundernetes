package main

import (
	"net"

	log "github.com/sirupsen/logrus"
)

func main() {
	addr, err := net.ResolveUDPAddr("udp", ":3075")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Starting UDP server on port %d", addr.Port)
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	buffer := make([]byte, 32)
	log.Info("UDP server is listening")
	for {
		count, remote, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Error(err)
			continue
		}
		log.Infof("Read %d bytes, from remote address %v, with value %q", count, remote, buffer[:count])
		if count > 2 && buffer[0] == 0xFF && buffer[1] == 0xFF {
			buffer[0] = 0x00
			buffer[1] = 0x00
			count, err := conn.WriteTo(buffer[:count], remote)
			if err != nil {
				log.Error(err)
				continue
			}
			log.Infof("Valid input, sent %d bytes response to remote address %v", count, remote)
		}
	}
}
