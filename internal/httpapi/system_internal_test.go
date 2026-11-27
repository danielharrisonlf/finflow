package httpapi

import (
	"net"
	"reflect"
	"testing"
)

func TestPrivateIPv4URLs(t *testing.T) {
	ips := []net.IP{
		net.ParseIP("192.168.0.10"),
		net.ParseIP("10.0.0.5"),
		net.ParseIP("172.20.1.2"),
		net.ParseIP("172.32.0.1"),  // fora de 172.16/12
		net.ParseIP("8.8.8.8"),     // público
		net.ParseIP("127.0.0.1"),   // loopback
		net.ParseIP("169.254.1.1"), // link-local
		net.ParseIP("fe80::1"),     // IPv6
		net.ParseIP("192.168.0.10"),
	}
	got := privateIPv4URLs(ips, 3333)
	want := []string{"http://10.0.0.5:3333", "http://172.20.1.2:3333", "http://192.168.0.10:3333"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("URLs = %v, want %v", got, want)
	}
	if got := privateIPv4URLs(nil, 1); got == nil || len(got) != 0 {
		t.Fatalf("sem IPs deve ser lista vazia (não nil): %v", got)
	}
}
