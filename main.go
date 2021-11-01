package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type NotifyMessage struct {
	Type int
	Data []byte
}

type Blockchain struct {
	PrevHashCode string
	HashCode     string
	Value        int64
	Timestamp    int64
}

type BlockPeer struct {
	chain            *Blockchain
	socket           *net.UDPConn
	HttpListenAddr   string
	SocketListenAddr string
	Peers            []*net.UDPConn
	ConnectedPeers   map[string]struct{}
}

func NewBlockPeer(httpAddr, p2pAddr string) *BlockPeer {
	peer := &BlockPeer{
		chain: &Blockchain{
			PrevHashCode: "0",
			HashCode:     "816534932c2b7154836da6afc367695e6337db8a921823784c14378abed4f7d7",
			Value:        0,
			Timestamp:    1465154705,
		},
		HttpListenAddr:   httpAddr,
		SocketListenAddr: p2pAddr,
		ConnectedPeers:   make(map[string]struct{}),
	}

	ipPorts := strings.Split(peer.SocketListenAddr, ":")
	ip := net.ParseIP(ipPorts[0])
	port, _ := strconv.Atoi(ipPorts[1])
	socket, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   ip,
		Port: port,
		Zone: "",
	})
	if err != nil {
		panic(err)
	}
	peer.socket = socket
	return peer
}

// start a block chain
func (b *BlockPeer) StartUp() {
	// start Listen
	go b.ReceiveMessage()
	// mine block
	http.HandleFunc("/mine", b.WrapperHttp(b.Mine))
	// add peers
	http.HandleFunc("/addPeers", b.AddPeers)
	// query block
	http.HandleFunc("/blocks", b.WrapperHttp(b.Blocks))
	http.ListenAndServe(b.HttpListenAddr, nil)
	select {}
}

// ReceiveMessage , receive message
func (b *BlockPeer) ReceiveMessage() (err error) {
	for {
		var (
			msg       [1024]byte
			notifyMsg NotifyMessage
			peers     []string
		)

		count, addr, errReceive := b.socket.ReadFrom(msg[:])
		if err != nil {
			print("接受数据错误", addr, errReceive)
			continue
		}

		// 解析数据
		data := msg[0:count]
		json.Unmarshal(data, &notifyMsg)
		switch notifyMsg.Type {
		// add peer
		case 1:
			json.Unmarshal(notifyMsg.Data, &peers)
			for _, peer := range peers {
				fmt.Println("接受到数据:", peer, time.Now().String())
				// 已经连接，则跳过
				if _, ok := b.ConnectedPeers[peer]; ok {
					fmt.Println(peer, "已经连接，跳过", time.Now().String())
					continue
				}

				// 连接peer
				connectedPeer, errConn := b.connectPeer(peer)
				if errConn != nil {
					continue
				}
				b.Peers = append(b.Peers, connectedPeer)
				b.ConnectedPeers[peer] = struct{}{}
				fmt.Println("success add peer", connectedPeer.RemoteAddr().String(), time.Now().String())

				// notify peer to add me
				peerData, _ := json.Marshal([]string{b.SocketListenAddr})
				peerMsg, _ := json.Marshal(&NotifyMessage{
					Type: 1,
					Data: peerData,
				})
				connectedPeer.Write(peerMsg)
				fmt.Println("通知对端连接", peer, "->", b.SocketListenAddr)
			}
		}
	}
}

// AddPeers add peers to this,body like :[{"ws://localhost:8081"}]
func (b *BlockPeer) AddPeers(writer http.ResponseWriter, req *http.Request) {
	// decode data
	var peers []string
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&peers); err != nil {
		writer.WriteHeader(http.StatusGone)
		return
	}
	data, _ := json.Marshal(peers)
	peerMessage := &NotifyMessage{
		Type: 1,
		Data: data,
	}
	msg, err := json.Marshal(peerMessage)
	if err != nil {
		writer.WriteHeader(http.StatusGone)
		return
	}

	// send message
	client, err := b.connectPeer(b.SocketListenAddr)
	if err != nil {
		writer.WriteHeader(http.StatusGone)
		return
	}
	if _, errWrite := client.Write(msg); errWrite != nil {
		writer.WriteHeader(http.StatusGone)
		return
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte("OK"))
}

// mine a new block
func (b *BlockPeer) Mine() {

}

// query blocks from chain
func (b *BlockPeer) Blocks() {

}

func (b *BlockPeer) WrapperHttp(fn func()) func(http.ResponseWriter, *http.Request) {
	return func(http.ResponseWriter, *http.Request) {
		fn()
	}
}

func (b *BlockPeer) connectPeer(addr string) (socket *net.UDPConn, err error) {
	ipPorts := strings.Split(addr, ":")
	ip := net.ParseIP(ipPorts[0])
	port, _ := strconv.Atoi(ipPorts[1])
	socket, err = net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   ip,
		Port: port,
	})
	return
}

func main() {
	httpAddr := flag.String("api", "127.0.0.1:3001", "api server address.")
	p2pAddr := flag.String("p2p", "127.0.0.1:6001", "p2p server address.")
	flag.Parse()

	ipPorts := strings.Split(*p2pAddr, ":")
	if ipPorts[0] == "localhost" || ipPorts[0] == "" {
		*p2pAddr = "127.0.0.1" + ":" + ipPorts[1]
	}
	fmt.Println("初始化区块...", "http:", *httpAddr, "p2p:", *p2pAddr)
	peer := NewBlockPeer(*httpAddr, *p2pAddr)

	fmt.Println("启动区块...")
	peer.StartUp()
}
