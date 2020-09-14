package arpdis

import (
	"errors"
	"net"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var (
	ipv4Id  uint16 = 1
	icmpSeq uint16 = 1
)

func NewIPv4Unreachable(src *Addr, dst *Addr) ([]byte, error) {
	ipv4Id++
	icmpSeq++

	ipv4 := layers.IPv4{
		Version: 4,
		IHL:     5,
		TOS:     0,
		// Length  :   uint16,
		Id:         ipv4Id,
		Flags:      0,
		FragOffset: 0,
		TTL:        10,
		Protocol:   layers.IPProtocolICMPv4,
		// Checksum :  uint16,
		SrcIP:   src.IP.To4(),
		DstIP:   dst.IP.To4(),
		Options: nil,
		Padding: nil,
	}

	icmp := layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeDestinationUnreachable, layers.ICMPv4CodeNet),
		Id:       uint16(os.Getpid()),
		Seq:      icmpSeq,
	}

	buf := gopacket.NewSerializeBuffer()
	err := gopacket.SerializeLayers(buf, defaultSerializeOpts, &ipv4, &icmp)

	return buf.Bytes(), err
}

const (
	ProtocolICMP     = 1
	ProtocolIPv6ICMP = 58
)

func doPing(ip string) error {
	raddr, _ := net.ResolveIPAddr("ip4:icmp", ip)
	conn, err := icmp.ListenPacket("ip4:icmp", "")
	if err != nil {
		return err
	}

	ipv4Conn := conn.IPv4PacketConn()
	// 限制跳跃数
	err = ipv4Conn.SetTTL(10)
	if err != nil {
		return err
	}

	msg := &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: timeToBytes(time.Now()),
		},
	}

	b, err := msg.Marshal(nil)
	if err != nil {
		return err
	}
	_, err = conn.WriteTo(b, raddr)
	if err != nil {
		return err
	}

	conn.SetReadDeadline(time.Now().Add(time.Second * 2))

	for {
		buf := make([]byte, 512)
		n, dst, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		if dst.String() != ip {
			continue
		}

		var result *icmp.Message
		result, err = icmp.ParseMessage(ProtocolICMP, buf[:n])
		if err != nil {
			return err
		}

		switch result.Type {
		case ipv4.ICMPTypeEchoReply:
			// success
			if rply, ok := result.Body.(*icmp.Echo); ok {
				_ = rply
				// log.Printf("%+v \n", rply)
			}
			return nil

		// case ipv4.ICMPTypeTimeExceeded:
		// case ipv4.ICMPTypeDestinationUnreachable:
		default:
			return errors.New("DestinationUnreachable")
		}
	}
}

func timeToBytes(t time.Time) []byte {
	nsec := t.UnixNano()
	b := make([]byte, 8)
	for i := uint8(0); i < 8; i++ {
		b[i] = byte((nsec >> ((7 - i) * 8)) & 0xff)
	}
	return b
}

func bytesToTime(b []byte) time.Time {
	var nsec int64
	for i := uint8(0); i < 8; i++ {
		nsec += int64(b[i]) << ((7 - i) * 8)
	}
	return time.Unix(nsec/1000000000, nsec%1000000000)
}
