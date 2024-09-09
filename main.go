package main

import (
	"fmt"
	"os"

	"github.com/swesdek/gotorrent-client/torrentfile"
)

func main() {
	fromPath := os.Args[1]
	toPath := os.Args[2]

	tf, err := torrentfile.Open(fromPath) // Открытие .torrent и считывание данных
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = tf.DownloadToFile(toPath) // Скачивание файла
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
