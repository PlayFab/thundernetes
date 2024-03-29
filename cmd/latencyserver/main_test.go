package main

import (
	"fmt"
	"net"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var server, client *net.UDPConn

var _ = Describe("Latency Server tests", func() {
	It("should send response to the sender", func() {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", 3070))
		Expect(err).To(Succeed())
		msg := []byte("\xFF\xFFhello")
		client.WriteTo(msg, addr)
		serverLoop(server)
		buffer := make([]byte, 32)
		count, _, err := client.ReadFromUDP(buffer)
		Expect(err).To(Succeed())
		Expect(count).To(Equal(len(msg)))
		Expect(buffer[:count]).To(Equal([]byte("\x00\x00hello")))
	})
	It("should ignore the first request and send a response for the second", func() {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", 3070))
		Expect(err).To(Succeed())
		msg1 := []byte("goodbye")
		msg2 := []byte("\xFF\xFFhello")
		client.WriteTo(msg1, addr)
		serverLoop(server)
		client.WriteTo(msg2, addr)
		serverLoop(server)
		buffer := make([]byte, 32)
		count, _, err := client.ReadFromUDP(buffer)
		Expect(err).To(Succeed())
		Expect(count).To(Equal(len(msg2)))
		Expect(buffer[:count]).To(Equal([]byte("\x00\x00hello")))
	})
})

func TestLatencyServer(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Latency Server Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")
	var err error
	server, err = createServer(3070)
	Expect(err).To(Succeed())
	client, err = createServer(3071)
	Expect(err).To(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := server.Close()
	Expect(err).To(Succeed())
	err = client.Close()
	Expect(err).To(Succeed())
})
