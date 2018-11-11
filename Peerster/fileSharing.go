package main

import(
    "fmt"
    "bufio"
    "crypto/sha256"
    "hash"
    "os"
    "io"
    "bytes"
)

func createEmptyFile(filename string) {
    _, err := os.Stat("Peerster/_SharedFiles/" + filename)

    if(os.IsNotExist(err)) {
        _, err2 := os.Create("Peerster/_SharedFiles/" + filename)
        isError(err2)
    } else if(err != nil) {
        isError(err)
    }
}

func writeChunkToFile(filename string, chunk []byte) {
    _, err := os.Stat("Peerster/_SharedFiles/" + filename)

    if(os.IsNotExist(err)) {
        os.Create("Peerster/_SharedFiles/" + filename)
    } else if(err != nil) {
        isError(err)
        return
    }

    f, err := os.OpenFile("Peerster/_SharedFiles/"+filename, os.O_APPEND|os.O_WRONLY, 0600)
    isError(err)

    defer f.Close()

    _, err = f.Write(chunk)
    isError(err)
}

func copyFileToDownloads(filename string) {
    _, err := os.Stat("Peerster/_SharedFiles/" + filename)

    if(err != nil) {
        isError(err)
        return
    }

    sourceFile, err := os.Open("Peerster/_SharedFiles/" + filename)
    isError(err)
    defer sourceFile.Close()

    destFile, err := os.Create("Peerster/_Downloads/" + filename)
    isError(err)
    defer destFile.Close()

    _, err = io.Copy(destFile, sourceFile)
    isError(err)
}

func computeHash(b []byte) []byte {
    var h hash.Hash
    h = sha256.New()
    h.Write(b)
    return h.Sum(nil)
}

func checkFilesForNextChunk(gossiper *Gossiper, filesBeingDownloaded []FileAndIndex, origin string, nextChunkHash string) ([]byte, bool) {
    var chunk []byte

    for i, fileAndIndex := range filesBeingDownloaded {
        f := gossiper.IndexedFiles[fileAndIndex.Metahash]
        chunk = getChunkByIndex(f.Name, fileAndIndex.NextIndex)
        chunk_hash_hex := bytesToHex(computeHash(chunk))

        if(chunk_hash_hex == nextChunkHash) {
            gossiper.RequestOriginToFileAndIndex[origin][i].NextIndex++
            return chunk, true
        }
    }
    return chunk, false
}

// returns index of file, isMetafile, nextChunkHash, isLastChunk
func getNextChunkHashToRequest(gossiper *Gossiper, fileOrigin string, hashValue string) (int, bool, string, bool) {
    var nextChunkHash string
    isMetafile := false
    indexOfFile := -1
    var metahash_hex string

    filesOfOrigin, exists := gossiper.RequestDestinationToFileAndIndex[fileOrigin]

    if(!exists) {
        return -1, false, nextChunkHash, false
    }

    // First check if we received a metafile
    for i, fileAndIndex := range filesOfOrigin {
        metahash_hex = fileAndIndex.Metahash
        if((fileAndIndex.NextIndex == -1) && (hashValue == metahash_hex)) {
            // return first chunkHash
            nextChunkHash = metahash_hex[0:2*HASH_SIZE]
            isMetafile = true
            return i, isMetafile, nextChunkHash, false
        }
    }



    // Otherwise we received a chunk, must range over all metafiles and see which chunk hash 
    for i, fileAndIndex := range filesOfOrigin {
        
        metafile_hex := fileAndIndex.Metafile
        currentIndex := fileAndIndex.NextIndex
        currentChunkHash := metafile_hex[(currentIndex * 2 * HASH_SIZE) : (currentIndex+1) * 2 * HASH_SIZE]

        if(currentChunkHash == hashValue) {

            index := currentIndex + 1

            if(len(metafile_hex) >  (index + 1) * 2 * HASH_SIZE) {
                nextChunkHash := metafile_hex[(index * 2 * HASH_SIZE) : (index+1) * 2 * HASH_SIZE]
                return i, false, nextChunkHash, false

            } else if(len(metafile_hex) ==  (index + 1) * 2 * HASH_SIZE) {
                nextChunkHash := metafile_hex[index * 2 * HASH_SIZE : (index+1) * 2 * HASH_SIZE]
                return i, false, nextChunkHash, false

            } else {
                return i, false, nextChunkHash, true
            }
        }
    }

    return indexOfFile, isMetafile, nextChunkHash, false
}

// returns index of file, isMetafile, nextChunk
func getNextChunkToRequest(gossiper *Gossiper, fileOrigin string, hashValue []byte) (int, bool, []byte) {
    var nextChunk []byte
    isMetafile := false
    indexOfFile := -1

    _, exists := gossiper.RequestDestinationToFileAndIndex[fileOrigin]

    if(!exists) {
        return -1, false, nextChunk
    }

    // First check if we received a metafile
    for i, fileAndIndex := range gossiper.RequestDestinationToFileAndIndex[fileOrigin] {
        if((fileAndIndex.NextIndex == -1) && bytes.Equal(hashValue, hexToBytes(fileAndIndex.Metahash))) {
            // return first chunk
            nextChunk = getChunkByIndex(gossiper.IndexedFiles[bytesToHex(hashValue)].Name, 0)
            isMetafile = true
            return i, isMetafile, nextChunk
        }
    }

    // Otherwise we received a chunk, must range over all metafiles and see which chunk hash 
    for i, fileAndIndex := range gossiper.RequestDestinationToFileAndIndex[fileOrigin] {
        
        filename := gossiper.IndexedFiles[fileAndIndex.Metahash].Name

        currentChunk := getChunkByIndex(filename, fileAndIndex.NextIndex-1)
        currentHash := computeHash(currentChunk)

        if(bytes.Equal(currentHash, hashValue)) {
            nextChunk = getChunkByIndex(filename, fileAndIndex.NextIndex)
            isMetafile = false
            return i, isMetafile, nextChunk
        }
    }

    return indexOfFile, isMetafile, nextChunk
}

func getChunkByIndex(filename string, index int) []byte {
    // Load file
    file, err := os.Open("Peerster/_SharedFiles/" + filename)
    isError(err)
    defer file.Close()

    reader := bufio.NewReader(file)

    _, err = file.Seek(int64(index * CHUNK_SIZE), 0)
    isError(err)

    chunk := make([]byte, CHUNK_SIZE) // a chunk of the file

    n, err := reader.Read(chunk)
    isError(err)

    return chunk[0:n]

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

    f.Metafile = bytesToHex(metafile)   

    // compute metahash
    h = sha256.New()
    h.Write(metafile)
    metahash := h.Sum(nil)

    f.Metahash = bytesToHex(metahash)

    return f
}