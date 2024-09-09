package handshake

import (
	"fmt"
	"io"
)

type Handshake struct {
	Pstr     string
	Infohash [20]byte
	PeerID   [20]byte
}

// Функция сериализации данных для передачи по сети
func (h *Handshake) Serialize() []byte {
	buf := make([]byte, len(h.Pstr)+49)       // Буфер, в который будет записываться вся информация для хендшейка
	buf[0] = byte(len(h.Pstr))                // Первый параметр, записанный в буфер - длина названия протокола
	curr := 1                                 // Переменная, с помощью которой буду идти по порядку данных для их записи
	curr += copy(buf[curr:], h.Pstr)          // Записываю название протокола
	curr += copy(buf[curr:], make([]byte, 8)) // 8 пустых байт - опциональные параметры, который не использую
	curr += copy(buf[curr:], h.Infohash[:])   // Записываю хэш торрента
	curr += copy(buf[curr:], h.PeerID[:])     // и ID своего пира
	return buf
}

// Функция считывания данных хендшейка
func Read(r io.Reader) (*Handshake, error) {
	bufLen := make([]byte, 1) // Переменная для первого параметра - длины названия протокола
	_, err := io.ReadFull(r, bufLen)
	if err != nil {
		return nil, err
	}
	pstrLen := int(bufLen[0]) // Первый параметр достается из байтового среза

	if pstrLen == 0 {
		return nil, fmt.Errorf("Pstr length cannot be 0!\n")
	}

	handshakeBuf := make([]byte, 48+pstrLen) // Буфер, для всех остальных параметров хендшейка
	_, err = io.ReadFull(r, handshakeBuf)
	if err != nil {
		return nil, err
	}

	var infoHash, peerID [20]byte // Переменные для извлечения параметров из буфера в итоговый объект

	copy(infoHash[:], handshakeBuf[pstrLen+8:pstrLen+8+20]) // Копирование данных в итоговый обьект
	copy(peerID[:], handshakeBuf[pstrLen+8+20:])

	h := Handshake{
		Pstr:     string(handshakeBuf[0:pstrLen]),
		Infohash: infoHash,
		PeerID:   peerID,
	}

	return &h, nil
}

// Инициализатор объекта хендшейка
func New(infohash [20]byte, peerID [20]byte) *Handshake {
	return &Handshake{
		Pstr:     "BitTorrent protocol",
		Infohash: infohash,
		PeerID:   peerID,
	}
}
