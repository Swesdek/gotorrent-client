package torrentfile

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"math/rand"
	"os"

	"github.com/jackpal/bencode-go"
	"github.com/swesdek/gotorrent-client/download"
)

// Порт клиента
const Port uint16 = 5919

// Объект с информацией о файле
type bencodeInfo struct { // Пример данных:
	Pieces      string `bencode:"pieces"`       // (Блок хешей каждой части файла)
	PieceLength int    `bencode:"piece length"` // i262144e
	Length      int    `bencode:"length"`       // i351272960e
	Name        string `bencode:"name"`         // debian-10.2.0-amd64-netinst.iso
}

// Объект с данными трекера и bencodeInfo
type bencodeTorrent struct { // Пример данных:
	Announce string      `bencode:"announce"` // http://bttracker.debian.org:6969
	Info     bencodeInfo `bencode:"info"`
}

// Объект со всеми данными торрент файла
type TorrentFile struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

// Функция для скачивания данных и упаковки их в файл
func (t *TorrentFile) DownloadToFile(path string) error {
	var peerID [20]byte
	_, err := rand.Read(peerID[:]) // В качестве собственного PeerID генерируется массив из 20 случайных байт
	if err != nil {
		return err
	}

	peers, err := t.requestPeers(peerID, Port) // Запрос пиров у торрент трекера
	if err != nil {
		return err
	}

	torrent := download.Torrent{ // Объект со всей информацией нужной для скачивания
		Peers:       peers,
		PeerID:      peerID,
		InfoHash:    t.InfoHash,
		PieceHashes: t.PieceHashes,
		PieceLength: t.PieceLength,
		Length:      t.Length,
		Name:        t.Name,
	}

	buf, err := torrent.Download() // Скачивание данных
	if err != nil {
		return err
	}

	outFile, err := os.Create(path) // Создание файла
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = outFile.Write(buf) // Запись данных в файл
	if err != nil {
		return err
	}
	return nil
}

// Функция для превращения данных из .torrent файла в объект TorrentFile
func Open(path string) (TorrentFile, error) {
	file, err := os.Open(path) // Открытие файла
	if err != nil {
		return TorrentFile{}, err
	}
	defer file.Close()

	bto := bencodeTorrent{}             // Инициализация обьекта для записи
	err = bencode.Unmarshal(file, &bto) // Форматирование и запись в обьект
	if err != nil {
		return TorrentFile{}, err
	}

	return bto.toTorrentFile() // Форматирование bencodeTorrent в TorrentFile
}

// Функция вычисления InfoHash
func (i *bencodeInfo) hash() ([20]byte, error) {
	var buf bytes.Buffer

	err := bencode.Marshal(&buf, *i) // Превращение данных из обьекта в байтовый массив

	if err != nil {
		return [20]byte{}, err
	}

	h := sha1.Sum(buf.Bytes()) // Вычисление хеша

	return h, nil
}

// Разделение Pieces на хеши частей файла
func (i *bencodeInfo) splitPieceHashes() ([][20]byte, error) {
	hashlen := 20
	buf := []byte(i.Pieces)

	if len(buf)%hashlen != 0 {
		err := fmt.Errorf("Recieved malformed pieces of length %d", len(buf))
		return nil, err
	}

	numHashes := len(buf) / hashlen
	hashes := make([][20]byte, numHashes)

	for i := 0; i < numHashes; i++ {
		copy(hashes[i][:], buf[i*hashlen:(i+1)*hashlen])
	}

	return hashes, nil
}

// Конвертация bencodeTorrent в TorrentFile
func (bto *bencodeTorrent) toTorrentFile() (TorrentFile, error) {
	infoHash, err := bto.Info.hash() // Хеш данных о файле
	if err != nil {
		return TorrentFile{}, err
	}

	pieceHashes, err := bto.Info.splitPieceHashes() // Хеши каждой части файла
	if err != nil {
		return TorrentFile{}, err
	}

	t := TorrentFile{
		Announce:    bto.Announce,
		InfoHash:    infoHash,
		PieceHashes: pieceHashes,
		PieceLength: bto.Info.PieceLength,
		Length:      bto.Info.Length,
		Name:        bto.Info.Name,
	}

	return t, nil
}
