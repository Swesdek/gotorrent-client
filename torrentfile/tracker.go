package torrentfile

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"
	"github.com/swesdek/gotorrent-client/peers"
)

// Объект данных из ответа трекера на запрос
type bencodeTrackerResp struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

// Создание ссылки для запроса на трекер
func (t *TorrentFile) buildTrackerURL(peerID [20]byte, port uint16) (string, error) {
	base, err := url.Parse(t.Announce) // Считывание ссылки на трекер из Announce
	if err != nil {
		return "", nil
	}
	params := url.Values{ // Параметры для запроса
		"info_hash":  []string{string(t.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.Length)},
	}

	base.RawQuery = params.Encode() // Создание ссылки с параметрами для запроса
	return base.String(), nil
}

// Запрос пиров у трекера
func (t *TorrentFile) requestPeers(peerID [20]byte, port uint16) ([]peers.Peer, error) {
	url, err := t.buildTrackerURL(peerID, port)
	if err != nil {
		return nil, err
	}

	c := &http.Client{Timeout: 15 * time.Second} // Создание http клиента

	res, err := c.Get(url) // Запрос на трекер
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	trackerRes := bencodeTrackerResp{}

	err = bencode.Unmarshal(res.Body, &trackerRes) // Запись ответа трекера
	if err != nil {
		return nil, err
	}

	return peers.Unmarshal([]byte(trackerRes.Peers))
}
