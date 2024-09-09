package message

import (
	"encoding/binary"
	"fmt"
	"io"
)

type messageID uint8

const (
	// Блокировка соединения
	MsgChoke messageID = 0

	// Разблокировка соединения
	MsgUnchoke messageID = 1

	// Заинтересованность в получении данных
	MsgInterested messageID = 2

	// Незаинтересованность в получении данных
	MsgNotInterested messageID = 3

	// Сообщение о том, что на этом устройстве скачалась определенная часть файла
	MsgHave messageID = 4

	// Уведомление пиру, какие части файла уже скачаны клиентом
	MsgBitfield messageID = 5

	// Запрос данных
	MsgRequest messageID = 6

	// Отправка данных файла в ответ на запрос от пира
	MsgPiece messageID = 7

	// Отмена запроса
	MsgCancel messageID = 8
)

type Message struct {
	ID      messageID
	Payload []byte
}

// Сериализация сообщения
func (m *Message) Serialize() []byte {
	if m == nil {
		return make([]byte, 4)
	}

	length := uint32(len(m.Payload) + 1)         // Длина сообщения
	buf := make([]byte, 4+length)                // Буфер с сериализированными данными
	binary.BigEndian.PutUint32(buf[0:4], length) // Запись длины сообщения в буфер
	buf[4] = byte(m.ID)                          // Запись ID сообщения
	copy(buf[5:], m.Payload)                     // Запись полезной нагрузки
	return buf
}

// Считывание данных от пира в обьект Message
func Read(r io.Reader) (*Message, error) {
	bufLen := make([]byte, 4) // Переменная для длины
	_, err := io.ReadFull(r, bufLen)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(bufLen) // Конвертация длины из байтового среза в uint32

	if length == 0 { // Если пэйлоада нет, то это сообщение о поддержке соединения
		return nil, nil
	}

	msgBuf := make([]byte, length) // Запись остатка сообщения
	_, err = io.ReadFull(r, msgBuf)
	if err != nil {
		return nil, err
	}

	m := Message{
		ID:      messageID(msgBuf[0]),
		Payload: msgBuf[1:],
	}

	return &m, nil
}

// Создание экземпляра обьекта Message для запроса части файла
func FormatRequest(index, begin, length int) *Message {
	payload := make([]byte, 12) // Создание пейлоада для записи
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin)) // Кодировка UINT32 в 4 байта
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))
	return &Message{
		ID:      MsgRequest,
		Payload: payload,
	}
}

// Создание экземпляра Message, сообщающего о том, что на этом устройстве скачалась определенная часть файла
func FormatHave(index int) *Message {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(index))
	return &Message{
		ID:      MsgHave,
		Payload: payload,
	}
}

// Считывание MsgHave и индекса куска файла от других пиров
func ParseHave(msg *Message) (int, error) {
	if msg.ID != MsgHave {
		return 0, fmt.Errorf("Expected to get %d, but got %d", MsgHave, msg.ID)
	}
	if len(msg.Payload) != 4 {
		return 0, fmt.Errorf("Expected payload length 4, but got length of %d", len(msg.Payload))
	}
	index := int(binary.BigEndian.Uint32(msg.Payload))
	return index, nil
}

// Считывание части файла, отправленного пиром
func ParsePiece(index int, buf []byte, msg *Message) (int, error) {
	if msg.ID != MsgPiece {
		return 0, fmt.Errorf("Expected piece (ID %d), but got ID %d", MsgPiece, msg.ID)
	}
	if len(msg.Payload) < 8 {
		return 0, fmt.Errorf("Payload is too short: < 8")
	}
	parsedIndex := int(binary.BigEndian.Uint32(msg.Payload[0:4])) // Индекс части файла
	if parsedIndex != index {
		return 0, fmt.Errorf("Indexes doesnt match: got %d instead of %d", index, parsedIndex)
	}
	begin := int(binary.BigEndian.Uint32(msg.Payload[4:8])) // Начало участка, к которому принадлежит часть файла
	if begin >= len(buf) {
		return 0, fmt.Errorf("Begin offset is too big: %d >= %d", begin, len(buf))
	}
	data := msg.Payload[8:] // Данные файла
	if begin+len(data) > len(buf) {
		return 0, fmt.Errorf("Data is too long [%d] for offset [%d] with length [%d]", len(data), begin, len(buf))
	}
	copy(buf[begin:], data) // Запись в общий буфер с данными файла
	return len(data), nil
}
