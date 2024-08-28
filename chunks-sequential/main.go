package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var mu sync.Mutex

func main() {
	http.HandleFunc("/upload", uploadHandler)

	// serve index.html
	http.Handle("/", http.FileServer(http.Dir(".")))

	fmt.Println("Server started at :8080")
	http.ListenAndServe(":8080", nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Retrieve the file name and starting byte
	fileName := r.FormValue("fileName")
	startByte, err := strconv.ParseInt(r.FormValue("start"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid start byte", http.StatusBadRequest)
		return
	}

	// Open the file in append mode
	mu.Lock()
	defer mu.Unlock()
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		http.Error(w, "Unable to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Seek to the correct offset
	if _, err := file.Seek(startByte, 0); err != nil {
		http.Error(w, "Unable to seek to offset", http.StatusInternalServerError)
		return
	}

	// Copy the file chunk to the target file
	_, err = io.Copy(file, r.Body)
	if err != nil {
		http.Error(w, "Unable to write file chunk", http.StatusInternalServerError)
		return
	}

	// Simulate a delay to test the chunking
	time.Sleep(1 * time.Second)

	fmt.Fprintf(w, "Chunk uploaded successfully")
}
