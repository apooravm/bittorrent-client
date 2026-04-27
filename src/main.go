package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strings"
	"time"

	bencodeparser "github.com/apooravm/bittorrent-client/src/bencode_parser"
)

type Peer struct {
	IP   net.IP
	Port uint16
	Conn net.Conn
	Id   string
}

var (
	peers            []Peer
	torrent_file_raw []byte
	parsed_torrent   bencodeparser.ParsedData
	torrent_filename = "example_ut.torrent"
	peerID           = []byte("-GO0001-123456789012")
	state            GlobalState
	piece_hashes     [][]byte
	piece_length     int64
	block_size       int = 16384
	block_count      int
	total_size       int64
	torrent_dir      TorrentDir
)

// so i had been treating pieaces like a list all this while
// but its just a big string, 20byte per piece, all joined together
func parse_piece_hashes(big_hash string) [][]byte {
	var hashes [][]byte

	for i := 0; i+20 <= len(big_hash); i += 20 {
		hashes = append(hashes, []byte(big_hash[i:i+20]))
	}

	return hashes
}

func main() {
	var err error
	parsed_torrent, err = bencodeparser.ParseFile(torrent_filename)
	if err != nil {
		log.Println(err.Error())
		return
	}

	res, ok := parsed_torrent.Data.(bencodeparser.ParsedDict)
	if !ok {
		return
	}

	piece_length_r, ok := res["info"].(bencodeparser.ParsedDict)["piece length"].(int)
	if !ok {
		fmt.Println("Could not cast piece length to int64")
		return
	}
	piece_length = int64(piece_length_r)

	block_count = int(piece_length / int64(block_size))

	// set hash pieces and initialize the empty bitfield
	pieces_arr := res["info"].(bencodeparser.ParsedDict)["pieces"].(string)
	piece_hashes = parse_piece_hashes(pieces_arr)

	state.InitBitfield(len(piece_hashes))

	torrent_files_r := res["info"].(bencodeparser.ParsedDict)["files"].(bencodeparser.ParsedList)
	var torrent_files []TorrentFileObj
	for _, f := range torrent_files_r {
		fileDict := f.(bencodeparser.ParsedDict)

		filepath_arr_raw := fileDict["path"].(bencodeparser.ParsedList)
		var filepath_parts []string

		for _, p := range filepath_arr_raw {
			str, ok := p.(string)
			if !ok {
				// handle error (unexpected type)
				continue
			}
			filepath_parts = append(filepath_parts, str)
		}

		file_length, ok := fileDict["length"].(int)
		if !ok {
			log.Println("E: Casting length to int")
			return
		}

		torrent_files = append(torrent_files, TorrentFileObj{
			Path:   filepath_parts,
			Length: int64(file_length),
		})
	}

	fmt.Println("All files add up to", total_size)
	fmt.Println("Piece length", piece_length)
	fmt.Println("Total piece length (piece_length * piece_count) piece_count = len(piece_hashes)", piece_length*int64(len(piece_hashes)))

	torrent_dir.Dirname = res["info"].(bencodeparser.ParsedDict)["name"].(string)
	torrent_dir.Files = torrent_files

	torrent_file_raw, err = os.ReadFile(torrent_filename)
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

	if err = announce_req(conn); err != nil {
		return
	}

	single_peer_handshake("127.0.0.1:6881")
	// multi_peer_handshake()
}

func single_peer_handshake(peer_addr string) {
	fmt.Println("Connecting to qbittorrent...")

	add_str := peer_addr
	p_conn, err := net.DialTimeout("tcp", add_str, 3*time.Second)
	if err != nil {
		log.Println("E: Connecting to peer", err.Error())
		return
	}
	p_conn.SetDeadline(time.Now().Add(30 * time.Second))

	defer p_conn.Close()

	fmt.Println("Connection successful")

	buf := make([]byte, 68)
	buf[0] = 19
	copy(buf[1:], "BitTorrent protocol")
	copy(buf[28:], parsed_torrent.Info_hash[:])
	copy(buf[48:], peerID)

	_, err = p_conn.Write(buf)
	if err != nil {
		log.Println("E: Writing to peer conn", err.Error())
		return
	}

	resp := make([]byte, 68)
	_, err = io.ReadFull(p_conn, resp)
	// _, err = p_conn.Read(resp)
	if err != nil {
		log.Println("E: Reading from peer conn", err.Error())
		return
	}

	fmt.Printf("Raw: %x\n", resp)
	fmt.Printf("Protocol: %s\n", resp[1:20])

	c1 := string(resp[1:20]) == "BitTorrent protocol"
	if !c1 {
		log.Println("Not Bittorrent protocol")
		return
	}

	c2 := bytes.Equal(resp[28:48], parsed_torrent.Info_hash[:])
	if !c2 {
		log.Println("Info hash dont match")
		return
	}

	peer_ID := resp[48:68]
	fmt.Println("Ready to transfer, peer_ID", string(peer_ID))

	// peer message structure after handshake
	// [length 4_BYTE][message_id 1_BYTE][payload ?_BYTE]

	// interested in transfer
	_, err = p_conn.Write([]byte{0, 0, 0, 1, 2})
	if err != nil {
		log.Println("E: Writing to conn 2.", err.Error())
		return
	}

	// Send bitfield here; even if empty
	// Packing bitfield; 8bits => 1 byte of of the arr being transferred
	packed_bitfield := state.GetPackedBitfield()
	payload_len := uint32(1 + len(packed_bitfield))
	bitfield_buf := make([]byte, 4+payload_len)
	binary.BigEndian.PutUint32(bitfield_buf[0:4], payload_len)
	bitfield_buf[4] = 5
	copy(bitfield_buf[5:], packed_bitfield)

	_, err = p_conn.Write(bitfield_buf)
	if err != nil {
		log.Println("E: Writing bitfield to conn", err.Error())
		return
	}

	is_unchoked := false
	for {
		// read length
		// read len of the resp first
		// since first 4 bytes are length
		lenBuf := make([]byte, 4)
		_, err := io.ReadFull(p_conn, lenBuf)
		if err != nil {
			log.Println("read len err:", err)
			return
		}

		length := binary.BigEndian.Uint32(lenBuf)

		// keep-alive
		if length == 0 {
			continue
		}

		msg := make([]byte, length)
		_, err = io.ReadFull(p_conn, msg)
		if err != nil {
			log.Println("read msg err:", err)
			return
		}

		msgID := msg[0]

		switch msgID {
		// piece bitfield
		case 5:
			fmt.Println("\ngot bitfield")
			fmt.Println(msg)

		// unchoke - transfer allowed
		case 1:
			if is_unchoked {
				continue
			}

			fmt.Println("UNCHOKED ✅")
			is_unchoked = true

			// Create all files if not exist
			if err = CreateAllFiles(torrent_dir); err != nil {
				return
			}

			for piece_idx := range len(piece_hashes) {
				// reset deadline
				p_conn.SetDeadline(time.Now().Add(120 * time.Second))

				thisPieceSize := piece_length
				curr_block_count := block_count

				remainder := total_size % int64(piece_length)

				if piece_idx == len(piece_hashes)-1 && remainder != 0 {
					thisPieceSize = remainder
					curr_block_count = int(thisPieceSize / int64(block_size))

				}
				lastBlockSize := thisPieceSize % int64(block_size)
				if lastBlockSize != 0 {
					curr_block_count += 1
				}

				piece := make([]byte, thisPieceSize)

				fmt.Printf("Downloading piece %v\n", piece_idx)
				fmt.Printf("Starting transfer blocks %v - %v\n", piece_idx, curr_block_count)

				for i := range curr_block_count {
					begin := i * block_size

					fmt.Println("Requesting block", i)

					req_buf := make([]byte, 17)
					binary.BigEndian.PutUint32(req_buf[0:4], 13)
					// msg ID
					req_buf[4] = 6
					// piece idx
					binary.BigEndian.PutUint32(req_buf[5:9], uint32(piece_idx))
					// byte offset within piece
					binary.BigEndian.PutUint32(req_buf[9:13], uint32(begin))
					// block size = 16384
					if i == curr_block_count-1 && lastBlockSize != 0 {
						// requestSize = lastBlockSize
						binary.BigEndian.PutUint32(req_buf[13:17], uint32(lastBlockSize))
					} else {
						binary.BigEndian.PutUint32(req_buf[13:17], uint32(block_size))
					}

					_, err = p_conn.Write(req_buf)
					if err != nil {
						fmt.Println("E: Writing piece request buf to peer conn", err.Error())
						return
					}

					// ------------------

					// instead of reading once, loop until you get the piece message you want
					var msg2 []byte
					for {
						lenBuf := make([]byte, 4)
						_, err := io.ReadFull(p_conn, lenBuf)
						if err != nil {
							log.Println("read len err:", err)
							return
						}
						length := binary.BigEndian.Uint32(lenBuf)
						if length == 0 {
							continue // keepalive
						}
						msg2 = make([]byte, length)
						_, err = io.ReadFull(p_conn, msg2)
						if err != nil {
							log.Println("read msg err:", err)
							return
						}
						if msg2[0] == 7 {
							break // got what we want
						}
						// handle or ignore anything else
						fmt.Println("skipping message during download:", msg2[0])
					}

					//----------------

					res_piece_idx := binary.BigEndian.Uint32(msg2[1:5])
					if res_piece_idx != uint32(piece_idx) {
						fmt.Printf("Invalid piece idx received. Expected %v, received %v\n", piece_idx, res_piece_idx)
						return
					}

					res_begin := binary.BigEndian.Uint32(msg2[5:9])
					if res_begin != uint32(begin) {
						fmt.Printf("Invalid begin offset received. Expected %v, received %v\n", begin, res_begin)
						return
					}

					copy(piece[begin:begin+block_size], msg2[9:])
					fmt.Printf("Block %v downloaded\n", i)
				}

				// compaire piece hash with expected
				piece_hash := sha1.Sum(piece)

				if !bytes.Equal(piece_hash[:], piece_hashes[piece_idx]) {
					fmt.Println("SHA1 mismatch")
					return
				}

				fmt.Println("Piece verified")
				state.SetBitAvailable(piece_idx)

				pieceStart := int64(piece_idx) * thisPieceSize
				pieceEnd := int64(pieceStart) + thisPieceSize

				var file_offset int64 = 0
				for _, f := range torrent_dir.Files {
					fileStart := file_offset
					fileEnd := fileStart + f.Length

					if pieceStart >= fileEnd {
						continue
					}

					if pieceEnd <= fileStart {
						continue
					}

					writeStart := util_max(pieceStart, fileStart)
					writeEnd := util_min(pieceEnd, fileEnd)

					offsetInPiece := writeStart - pieceStart
					offsetInFile := writeStart - fileStart

					fl, err := os.OpenFile(f.JoinedPath, os.O_RDWR, 0644)
					if err != nil {
						log.Println("E: Opening file during piece write", err.Error())
						return
					}

					_, err = fl.WriteAt(piece[offsetInPiece:offsetInPiece+(writeEnd-writeStart)], offsetInFile)
					if err != nil {
						log.Println("E: Writing piece to file", err.Error())
					}

					fl.Close()

					file_offset += f.Length
				}
			}

		// choke - cannot transfer
		case 0:
			fmt.Println("choked")

		// have piece
		case 4:
			fmt.Println("have message")

		default:
			fmt.Println("other:", msgID)
		}
	}
}

func multi_peer_handshake() {
	// peer handshake
	for i := range len(peers) {
		ip_str := peers[i].IP.String()
		port_str := fmt.Sprintf("%d", peers[i].Port)
		addr_str := fmt.Sprintf("%s:%s", ip_str, port_str)

		fmt.Printf("Dialing %s:%s\n", ip_str, port_str)

		p_conn, err := net.DialTimeout("tcp", addr_str, 500*time.Millisecond)
		if err != nil {
			log.Println("E: Connecting to peer", err.Error())
			continue
		}

		fmt.Println("Connection successful")

		// p_conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		buf := make([]byte, 68)
		buf[0] = 19
		copy(buf[1:], "BitTorrent protocol")
		copy(buf[28:], parsed_torrent.Info_hash[:])
		copy(buf[48:], peerID)

		_, err = p_conn.Write(buf)
		if err != nil {
			log.Println("E: Writing to peer conn", err.Error())
			return
		}

		resp := make([]byte, 68)
		_, err = io.ReadFull(p_conn, resp)
		// _, err = p_conn.Read(resp)
		if err != nil {
			log.Println("E: Reading from peer conn", err.Error())
			continue
		}

		fmt.Printf("Raw: %x\n", resp)
		fmt.Printf("Protocol: %s\n", resp[1:20])

		p_conn.Close()
		break
	}
}

// construct announce_req and fetch peer list
func announce_req(conn *net.UDPConn) error {
	var err error

	resp := make([]byte, 16)
	_, err = conn.Read(resp)
	if err != nil {
		log.Println("E: Reading from UDP conn", err.Error())
		return err
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

	copy(buf[16:36], parsed_torrent.Info_hash[:])

	// peer_id (20 bytes)
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
		return err
	}

	resp = make([]byte, 1500)
	n, err := conn.Read(resp)
	if err != nil {
		log.Println("E: Reading resp from conn", err.Error())
		return err
	}

	for i := 20; i < n; i += 6 {
		ip := net.IP(resp[i : i+4])
		port := binary.BigEndian.Uint16(resp[i+4 : i+6])

		peers = append(peers, Peer{IP: ip, Port: port})
	}

	fmt.Printf("%d peers found\n", len(peers))

	return nil
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
