package client

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/swesdek/gotorrent-client/bitfields"
	"github.com/swesdek/gotorrent-client/handshake"
	"github.com/swesdek/gotorrent-client/message"
	"github.com/swesdek/gotorrent-client/peers"
)

type Client struct {
	Conn     net.Conn
	Choked   bool
	Bitfield bitfields.Bitfield
	peer     peers.Peer
	InfoHash [20]byte
	PeerID   [20]byte
}

// Выполнение рукопожатия с другими пирами
func completeHandshake(conn net.Conn, infohash [20]byte, peerID [20]byte) (*handshake.Handshake, error) {
	conn.SetDeadline(time.Now().Add(time.Second * 3)) // Дедлайн ожидания ответа от пира
	defer conn.SetDeadline(time.Time{})               // Отмена дедлайна

	req := handshake.New(infohash, peerID) // Создание объекта хендшейка
	_, err := conn.Write(req.Serialize())  // Отправка пиру хендшейка
	if err != nil {
		return nil, err
	}

	res, err := handshake.Read(conn) // Считывание ответа пира
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(infohash[:], res.Infohash[:]) { // Сравнение хешей торрента
		return nil, fmt.Errorf("Expected %x, but got %x insted", infohash, res.Infohash)
	}
	return res, nil
}

// Функция получения информации об имеющихся у пира частях файла
func getBitfield(conn net.Conn) (bitfields.Bitfield, error) {
	conn.SetDeadline(time.Now().Add(time.Second * 5)) // Дедлайн ожидания ответа от пира
	defer conn.SetDeadline(time.Now())                // Отмена дедлайна

	msg, err := message.Read(conn) // Считывание данных от пира
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, fmt.Errorf("Expected bitfield but got nothing")
	}
	if msg.ID != message.MsgBitfield {
		return nil, fmt.Errorf("Expected bitfield ID, but got %d", msg.ID)
	}

	return msg.Payload, nil
}

// Инициализатор объекта клиента
func New(peer peers.Peer, peerID, infoHash [20]byte) (*Client, error) {
	conn, err := net.DialTimeout("tcp", peer.String(), 3*time.Second) // Установление TCP соединения с клиентом
	if err != nil {
		return nil, err
	}

	_, err = completeHandshake(conn, infoHash, peerID) // Хендшейк
	if err != nil {
		conn.Close()
		return nil, err
	}

	bf, err := getBitfield(conn) // Получение информации об имеющихся частях файла
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Client{
		Conn:     conn,
		Choked:   true,
		Bitfield: bf,
		peer:     peer,
		InfoHash: infoHash,
		PeerID:   peerID,
	}, nil
}

// Функция считывания информации с соединения
func (c *Client) Read() (*message.Message, error) {
	msg, err := message.Read(c.Conn)
	return msg, err
}

// Функция отправления запроса на кусок файла
func (c *Client) SendRequest(index, begin, length int) error {
	req := message.FormatRequest(index, begin, length)
	_, err := c.Conn.Write(req.Serialize())
	return err
}

// Функция для отправки сообщения о разблокировке соединения
func (c *Client) SendUnchoke() error {
	msg := message.Message{ID: message.MsgUnchoke}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// Функция для отправки сообщения о заинтересованности в получении данных
func (c *Client) SendInterested() error {
	msg := message.Message{ID: message.MsgInterested}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// Функция для отправки сообщения о не заинтересованности в получении данных
func (c *Client) SendNotInterested() error {
	msg := message.Message{ID: message.MsgNotInterested}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// Функция для отправки сообщения с вопросом, есть ли у пира нужная часть файла
func (c *Client) SendHave(index int) error {
	msg := message.FormatHave(index)
	_, err := c.Conn.Write(msg.Serialize())
	return err
}
