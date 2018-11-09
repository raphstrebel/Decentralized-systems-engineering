package main

import(
    "fmt"
    "bufio"
    "crypto/sha256"
    "hash"
    "os"
    "io"
)

func computeHash(b []byte) []byte {
    var h hash.Hash
    h = sha256.New()
    h.Write(b)
    return h.Sum(nil)
}

func checkFilesForHash(gossiper *Gossiper, filesBeingDownloaded []FileAndIndex, origin string, nextChunkHash string) Chunk {
    var chunk Chunk
    var h hash.Hash

    for i, fileAndIndex := range filesBeingDownloaded {
        f := gossiper.IndexedFiles[fileAndIndex.Metahash]
        chunk = getChunkByIndex(f.Name, fileAndIndex.NextIndex)

        h = sha256.New()
        h.Write(chunk.ByteArray)
        chunk_hash_hex := bytesToHex(h.Sum(nil))

        if(chunk_hash_hex == nextChunkHash) {
            gossiper.nodeToFilesDownloaded[origin][i].NextIndex++
            return chunk
        }
    }

    return chunk
}

func getChunkByIndex(filename string, index int) Chunk {
    // Load file
    file, err := os.Open("Peerster/_SharedFiles/" + filename)
    isError(err)
    defer file.Close()

    reader := bufio.NewReader(file)

    _, err = file.Seek(int64(index * CHUNK_SIZE), 0)
    isError(err)

    chunk := make([]byte, CHUNK_SIZE) // a chunk of the file

    _, err = reader.Read(chunk)
    isError(err)

    return Chunk{ByteArray: chunk}

}

func computeFileIndices(filename string) File {
    // Should we initialize the rest of the fields with make([]byte, ...) ? 
    f := File{Name: filename}

    // Load file
    file, err := os.Open("Peerster/_SharedFiles/" + filename)
    isError(err)
    defer file.Close()

    // Get file length
    fileStat, err := file.Stat()
    isError(err)
    f.Size = int(fileStat.Size())

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