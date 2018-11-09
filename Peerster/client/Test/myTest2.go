package main

import(
    "fmt"
    "os"
)

// Might be useful : https://gist.github.com/minikomi/2900454

func isError(err error) {
    if err  != nil {
        fmt.Println("An error occurred : " , err)
    }
}

func main() {

	filename := "file.txt"

    // Load file
    file, err := os.Open("Peerster/_SharedFiles/" + filename)
    isError(err)

    fmt.Println(file)

    // Get file length
    fileStat, err := file.Stat()
    isError(err)
    size := int(fileStat.Size())

    fmt.Println("The file is bytes long : ", size)

    defer file.Close()

}