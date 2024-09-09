package peers

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

type Peer struct {
	IP   net.IP
	Port uint16
}

// Конвертация байтового среза с пирами в объект Peer
func Unmarshal(peersBinary []byte) ([]Peer, error) {
	const peerSize = 6 // Размер данных одного пира (4 на IP, 2 на порт)
	numPeers := len(peersBinary) / peerSize
	if len(peersBinary)%peerSize != 0 {
		return nil, fmt.Errorf("Recieved malformed peers info")
	}

	peers := make([]Peer, numPeers) // Создание среза для пиров
	for i := 0; i < numPeers; i++ {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBinary[offset : offset+4])                      // Запись IP с помощью объекта net.IP
		peers[i].Port = binary.BigEndian.Uint16(peersBinary[offset+4 : offset+6]) // Запись порта
	}

	return peers, nil
}

// Склеивание IP и порта в одну строку
func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}
