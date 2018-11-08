package main

import(
    "fmt"
    "bufio"
    "crypto/sha256"
    "hash"
    "os"
    "io"
)

type Chunk struct {
    ByteArray []byte
}

type File struct {
    Name string
    Size int
    Metafile []byte
    Metahash []byte
}

func computeFileIndices(filename string) File {
    CHUNK_SIZE := 1024*8

    // Should we initialize the rest of the fields with make([]byte, ...) ? 
    f := File{Name: filename}

    // Load file
    file, err := os.Open("Peerster/client/_SharedFiles/" + filename)
    isError(err)

    // Get file length
    fileStat, err := file.Stat()
    isError(err)
    f.Size = int(fileStat.Size())

    defer file.Close()

    reader := bufio.NewReader(file)
    chunk := make([]byte, CHUNK_SIZE) // a chunk of the file

    nbChunks := 0
    alreadyRead := 0

    // Compute SHA256 of all chunks
    sha_chunks := []Chunk{} // array of sha hashes of chunks
    var h hash.Hash
    metafile := []byte{}
    var chunk_hash []byte

    for {
        if _, err = reader.Read(chunk); err != nil {
            break
        }

        if(alreadyRead + CHUNK_SIZE < int(f.Size)) {
            h = sha256.New()
            h.Write(chunk)
            chunk_hash = h.Sum(nil)
            sha_chunks = append(sha_chunks, Chunk{ByteArray: chunk_hash})
            metafile = append(metafile, chunk_hash...)
            alreadyRead += CHUNK_SIZE

        } else { // this is the last chunk
            chunkLimit := int(f.Size) - alreadyRead
            h = sha256.New()
            h.Write(chunk[0:chunkLimit])

            chunk_hash = h.Sum(nil)
            sha_chunks = append(sha_chunks, Chunk{ByteArray: chunk_hash})
            metafile = append(metafile, chunk_hash...)
        }

        nbChunks++
    }

    if err != io.EOF {
        fmt.Println("Error Reading : ", err)    
    } else {
        err = nil
    }

    f.Metafile = metafile    

    // compute metahash
    h = sha256.New()
    h.Write(metafile)
    metahash := h.Sum(nil)

    f.Metahash = metahash

    return f
}