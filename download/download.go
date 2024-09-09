package download

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/swesdek/gotorrent-client/client"
	"github.com/swesdek/gotorrent-client/message"
	"github.com/swesdek/gotorrent-client/peers"
)

type Torrent struct {
	Peers       []peers.Peer
	PeerID      [20]byte
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

// Объект части файла
type pieceWork struct {
	index  int
	hash   [20]byte
	length int
}

// Скачанная часть файла
type pieceResult struct {
	index int
	buf   []byte
}

// State-объект
type pieceProgress struct {
	index      int
	client     *client.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}

const MaxBacklog = 5       // Максимальное количество неудовлетворенных запросов
const MaxBlockSize = 16384 // Максимальная длина блока данных

// Считывание ответа пира
func (state *pieceProgress) readMessage() error {
	msg, err := state.client.Read()
	if err != nil {
		return err
	}

	if msg == nil {
		return nil
	}

	switch msg.ID {
	case message.MsgUnchoke:
		state.client.Choked = false
	case message.MsgChoke:
		state.client.Choked = true
	case message.MsgHave:
		index, err := message.ParseHave(msg) // Считывание, какая часть файла есть у пира
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index) // Запись в Bitfield о содержании пиром соответствующей части
	case message.MsgPiece:
		n, err := message.ParsePiece(state.index, state.buf, msg) // Запись части файла, пришедшей от пира
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--
	}
	return nil
}

// Функция для отправки запроса на получение части файла
func tryDownloadPiece(c *client.Client, pw *pieceWork) ([]byte, error) {
	state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}

	c.Conn.SetDeadline(time.Now().Add(time.Second * 30)) // Установка дедлайна на ответ от пира
	defer c.Conn.SetDeadline(time.Now())

	for state.downloaded < pw.length {
		if !state.client.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.length {
				blockSize := MaxBlockSize

				if pw.length-state.requested < blockSize {
					blockSize = pw.length - state.requested
				}

				err := c.SendRequest(state.index, state.requested, blockSize) // Отправка запроса на пиры
				if err != nil {
					return nil, err
				}
				state.backlog += 1
				state.requested += blockSize
			}
		}

		err := state.readMessage() // Считывание ответа пира
		if err != nil {
			return nil, err
		}
	}

	return state.buf, nil
}

// Проверка части файла на цельность и соответствие запрошенному
func checkIntegrity(pw *pieceWork, buf []byte) error {
	hash := sha1.Sum(buf)                  // Вычисление хеша полученной части
	if !bytes.Equal(hash[:], pw.hash[:]) { // Сравнение хешей
		return fmt.Errorf("Piece %d failed to pass integrity check\n", pw.index)
	}
	return nil
}

// Установление соединения с пиром и запуск скачивания
func (t *Torrent) startDownloadWorker(peer peers.Peer, workQueue chan *pieceWork, results chan *pieceResult) {
	c, err := client.New(peer, t.PeerID, t.InfoHash) // Создание обьекта клиента
	if err != nil {
		fmt.Printf("Handshake with %s was unsuccessful. Shutting down worker\n", peer.IP)

		return
	}
	defer c.Conn.Close()

	c.SendUnchoke()    // Сообщение о разблокировке
	c.SendInterested() // Сообщение о заинтересованности в получении данных

	for pw := range workQueue {
		if !c.Bitfield.HasPiece(pw.index) { // Если у пира нет нужной части данных, то она помещается обратно в очередь
			workQueue <- pw
			continue
		}

		buf, err := tryDownloadPiece(c, pw) // Скачивание данных
		if err != nil {
			fmt.Println("Couldnt download piece from this peer. Exiting")
			workQueue <- pw
			return
		}

		err = checkIntegrity(pw, buf) // Проверка на цельность
		if err != nil {
			fmt.Println(err)
			workQueue <- pw
			return
		}

		c.SendHave(pw.index)                   // Сообщение пирам о завершении скачивания части файла
		results <- &pieceResult{pw.index, buf} // Помещение части файла в канал
	}
}

// Скачивание всего файла
func (t *Torrent) Download() ([]byte, error) {
	fmt.Printf("Starting download for %s\n", t.Name)

	workQueue := make(chan *pieceWork, len(t.PieceHashes)) // Очередь с данными о частях для скачивания
	results := make(chan *pieceResult)                     // Канал с готовыми для записи в файл частями

	// Заполнение очереди данными
	for index, hash := range t.PieceHashes {
		begin := index * t.PieceLength
		end := begin + t.PieceLength

		if end > t.Length {
			end = t.Length
		}

		length := end - begin

		workQueue <- &pieceWork{index, hash, length}
	}

	// Запуск многопоточного скачивания
	for _, peer := range t.Peers {
		go t.startDownloadWorker(peer, workQueue, results)
	}

	// Создание индикатора загрузки
	bar := progressbar.Default(int64(len(t.PieceHashes)))

	// Запись в итоговый буфер
	buf := make([]byte, t.Length)
	donePieces := 0
	for donePieces < len(t.PieceHashes) {
		res := <-results
		begin := res.index * t.PieceLength
		end := begin + t.PieceLength

		if end > t.Length {
			end = t.Length
		}

		copy(buf[begin:end], res.buf)
		donePieces += 1
		bar.Add(1)
	}

	close(workQueue) // Закрытие канала с очередью

	return buf, nil
}
