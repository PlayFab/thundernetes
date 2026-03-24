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
	It("getResponse with count <= 2 should return nil", func() {
		result := getResponse([]byte{0xFF, 0xFF}, 2)
		Expect(result).To(BeNil())
	})
	It("getResponse with count == 0 should return nil", func() {
		result := getResponse([]byte{}, 0)
		Expect(result).To(BeNil())
	})
	It("getResponse with non-0xFF prefix should return nil", func() {
		result := getResponse([]byte{0x01, 0x02, 0x03}, 3)
		Expect(result).To(BeNil())
	})
	It("getResponse with first byte 0xFF but second not should return nil", func() {
		result := getResponse([]byte{0xFF, 0x02, 0x03}, 3)
		Expect(result).To(BeNil())
	})
	It("getResponse with valid input should flip bytes", func() {
		result := getResponse([]byte{0xFF, 0xFF, 0x01, 0x02}, 4)
		Expect(result).NotTo(BeNil())
		Expect(result[0]).To(Equal(byte(0x00)))
		Expect(result[1]).To(Equal(byte(0x00)))
		Expect(result[2]).To(Equal(byte(0x01)))
		Expect(result[3]).To(Equal(byte(0x02)))
	})
	It("getResponse with minimum valid input (3 bytes) should flip bytes", func() {
		result := getResponse([]byte{0xFF, 0xFF, 0x01}, 3)
		Expect(result).NotTo(BeNil())
		Expect(result[0]).To(Equal(byte(0x00)))
		Expect(result[1]).To(Equal(byte(0x00)))
		Expect(result[2]).To(Equal(byte(0x01)))
	})
	It("createServer with valid port should create a server", func() {
		conn, err := createServer(3072)
		Expect(err).To(Succeed())
		Expect(conn).NotTo(BeNil())
		err = conn.Close()
		Expect(err).To(Succeed())
	})
	It("createServer with port 0 should succeed and OS picks a port", func() {
		conn, err := createServer(0)
		Expect(err).To(Succeed())
		Expect(conn).NotTo(BeNil())
		addr := conn.LocalAddr().(*net.UDPAddr)
		Expect(addr.Port).To(BeNumerically(">", 0))
		err = conn.Close()
		Expect(err).To(Succeed())
	})
	DescribeTable("getResponse edge cases",
		func(buffer []byte, count int, expectNil bool) {
			result := getResponse(buffer, count)
			if expectNil {
				Expect(result).To(BeNil())
			} else {
				Expect(result).NotTo(BeNil())
				Expect(result[0]).To(Equal(byte(0x00)))
				Expect(result[1]).To(Equal(byte(0x00)))
			}
		},
		Entry("count=1 with 0xFF byte returns nil (too short)", []byte{0xFF}, 1, true),
		Entry("large buffer (32 bytes) with valid prefix returns flipped", func() []byte {
			buf := make([]byte, 32)
			buf[0] = 0xFF
			buf[1] = 0xFF
			for i := 2; i < 32; i++ {
				buf[i] = byte(i)
			}
			return buf
		}(), 32, false),
		Entry("first byte 0xFF but second 0x00 returns nil", []byte{0xFF, 0x00, 0x03}, 3, true),
	)
	It("serverLoop should process a valid message and return the remote address", func() {
		testServer, err := createServer(0)
		Expect(err).To(Succeed())
		defer testServer.Close()

		testClient, err := createServer(0)
		Expect(err).To(Succeed())
		defer testClient.Close()

		serverAddr := testServer.LocalAddr().(*net.UDPAddr)
		msg := []byte("\xFF\xFFtest")
		_, err = testClient.WriteTo(msg, serverAddr)
		Expect(err).To(Succeed())

		remote, err := serverLoop(testServer)
		Expect(err).To(Succeed())
		Expect(remote).NotTo(BeNil())

		buffer := make([]byte, 32)
		count, _, err := testClient.ReadFromUDP(buffer)
		Expect(err).To(Succeed())
		Expect(count).To(Equal(len(msg)))
		Expect(buffer[:count]).To(Equal([]byte("\x00\x00test")))
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
