package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strings"

	bencodeparser "github.com/apooravm/bittorrent-client/src/bencode_parser"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

func main() {
	filename := "example_ut.torrent"
	data, err := bencodeparser.ParseFile(filename)
	if err != nil {
		log.Println(err.Error())
		return
	}

	res, ok := data.Data.(bencodeparser.ParsedDict)
	if !ok {
		return
	}

	fmt.Println("Announce", res["announce"])
	for k, _ := range res["info"].(bencodeparser.ParsedDict) {
		fmt.Println(k)
	}
	fmt.Println("Piece Length ->", res["info"].(bencodeparser.ParsedDict)["piece length"])

	fileData, err := os.ReadFile(filename)
	if err != nil {
		log.Println("E: Opening file.", err.Error())
		return
	}

	announce_url := res["announce"].(string)
	conn, err := connect_req(announce_url)
	if err != nil {
		return
	}

	defer conn.Close()

	resp := make([]byte, 16)
	_, err = conn.Read(resp)
	if err != nil {
		log.Println("E: Reading from UDP conn", err.Error())
		return
	}

	action := binary.BigEndian.Uint32(resp[0:4])
	txn_id := binary.BigEndian.Uint32(resp[4:8])
	conn_id := binary.BigEndian.Uint64(resp[8:16])

	fmt.Println("Action", action)
	fmt.Println("transaction_ID", txn_id)
	fmt.Println("connection_id", conn_id)

	buf := make([]byte, 98)

	binary.BigEndian.PutUint64(buf[0:8], conn_id)
	// action -> 1 (announce)
	binary.BigEndian.PutUint32(buf[8:12], 1)

	binary.BigEndian.PutUint32(buf[12:16], txn_id)

	copy(buf[16:36], fileData[data.Info_idx_start:data.Info_idx_end])

	// peer_id (20 bytes)
	peerID := []byte("-GO0001-123456789012")
	copy(buf[36:56], peerID)

	// downloaded
	binary.BigEndian.PutUint64(buf[56:64], 0)

	// left
	binary.BigEndian.PutUint64(buf[64:72], 0)

	// uploaded
	binary.BigEndian.PutUint64(buf[72:80], 0)

	// event (2 = started)
	binary.BigEndian.PutUint32(buf[80:84], 2)

	// IP address (0 = default)
	binary.BigEndian.PutUint32(buf[84:88], 0)

	// key (random)
	binary.BigEndian.PutUint32(buf[88:92], rand.Uint32())

	// num_want (-1)
	binary.BigEndian.PutUint32(buf[92:96], 0xFFFFFFFF)

	// port
	binary.BigEndian.PutUint16(buf[96:98], 6881)

	_, err = conn.Write(buf)
	if err != nil {
		log.Println("E: Writing announce to conn", err.Error())
		return
	}

	resp = make([]byte, 1500)
	n, err := conn.Read(resp)
	if err != nil {
		log.Println("E: Reading resp from conn", err.Error())
		return
	}

	for i := 20; i < n; i += 6 {
		ip := net.IP(resp[i : i+4])
		port := binary.BigEndian.Uint16(resp[i+4 : i+6])

		println(ip.String(), port)
	}
}

// connect_request -> get connection_id
// announce_req -> send torrent info, get peers
func connect_req(announce_url string) (*net.UDPConn, error) {
	// ONLY accepts host:port urls, not full (atleast for UDP trackers)
	clean := strings.TrimPrefix(announce_url, "udp://")
	clean = strings.TrimSuffix(clean, "/announce")

	addr, err := net.ResolveUDPAddr("udp", clean)
	if err != nil {
		log.Println("E: UDP addr could not be resolved", err.Error())
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Println("E: Dialing to UDP addr", err.Error())
		return nil, err
	}

	// Sending a connect_request
	buf := make([]byte, 16)

	// protocol ID
	binary.BigEndian.PutUint64(buf[0:8], 0x41727101980)

	// action; 0 -> connect
	binary.BigEndian.PutUint32(buf[8:12], 0)

	// transaction_ID (random)
	binary.BigEndian.PutUint32(buf[12:16], 12345)

	_, err = conn.Write(buf)
	if err != nil {
		log.Println("E: Writing to UDP conn", err.Error())
		return nil, err
	}

	return conn, nil
}
