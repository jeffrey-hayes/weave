package weavedns

import (
	"github.com/miekg/dns"
	"log"
	"net"
	"testing"
	"time"
)

var (
	container_id = "deadbeef"
	test_addr1   = "10.0.2.1/24"
)

func sendQuery(name string, querytype uint16) error {
	m := new(dns.Msg)
	m.SetQuestion(name, querytype)
	m.RecursionDesired = false
	buf, err := m.Pack()
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return err
	}
	_, err = conn.WriteTo(buf, ipv4Addr)
	return err
}

func TestServerSimpleQuery(t *testing.T) {
	log.Println("TestServerSimpleQuery starting")
	var zone = new(ZoneDb)
	docker_ip := net.ParseIP("9.8.7.6")
	weave_ip, subnet, _ := net.ParseCIDR(test_addr1)
	zone.AddRecord(container_id, "test.weave.", docker_ip, weave_ip, subnet)

	mdnsServer, err := NewMDNSServer(zone)
	assertNoErr(t, err)
	err = mdnsServer.Start(nil)
	assertNoErr(t, err)

	var received_addr net.IP
	received_count := 0

	// Implement a minimal listener for responses
	multicast, err := LinkLocalMulticastListener(nil)
	assertNoErr(t, err)

	handleMDNS := func(w dns.ResponseWriter, r *dns.Msg) {
		// Only handle responses here
		if len(r.Answer) > 0 {
			for _, answer := range r.Answer {
				switch rr := answer.(type) {
				case *dns.A:
					received_addr = rr.A
					received_count++
				}
			}
		}
	}

	server := &dns.Server{Listener: nil, PacketConn: multicast, Handler: dns.HandlerFunc(handleMDNS)}
	go server.ActivateAndServe()
	defer server.Shutdown()

	time.Sleep(100 * time.Millisecond) // Allow for server to get going

	sendQuery("test.weave.", dns.TypeA)

	time.Sleep(time.Second)

	if received_count != 1 {
		t.Log("Unexpected result count for test.weave", received_count)
		t.Fail()
	}
	if !received_addr.Equal(weave_ip) {
		t.Log("Unexpected result for test.weave", received_addr)
		t.Fail()
	}

	received_count = 0

	sendQuery("testfail.weave.", dns.TypeA)

	if received_count != 0 {
		t.Log("Unexpected result count for testfail.weave", received_count)
		t.Fail()
	}
}
