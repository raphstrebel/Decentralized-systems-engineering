package main

import(
    "fmt"
    "bufio"
    "crypto/sha256"
    "hash"
    "os"
    "io"
    "bytes"
    "regexp"
)

func getNbChunksFromMetafile(metafile string) uint64 {
    re := regexp.MustCompile(`[a-f0-9]{64}`)
    metafileArray := re.FindAllString(metafile, -1) // split metafile into array of 64 char chunks
    return uint64(len(metafileArray))
}

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

func getChunkMap(file File) []uint64 {
    var chunkMap []uint64

    // If we don't even have the first chunk
    if(file.NextIndex == -1 || file.NextIndex == 0) {
        fmt.Println("Next index of file is -1 or 0")
        return chunkMap
    } 

    for i := 0; i < file.NextIndex; i++ {
        chunkMap = append(chunkMap, uint64(i))
    }

    return chunkMap
}

// return next chunk, found (boolean), metahash
func checkFilesForNextChunk(origin string, nextChunkHash string) (string, bool, string) {
    var chunk string

    gossiper.SafeIndexedFiles.mux.Lock()

    // go over all metafiles

    for _, file := range gossiper.SafeIndexedFiles.IndexedFiles { 

        metafile := file.Metafile
        index := 0

        re := regexp.MustCompile(`[a-f0-9]{64}`)
        metafileArray := re.FindAllString(metafile, -1) // split metafile into array of 64 char chunks

        for _,chunkHash := range metafileArray {
            if(chunkHash == nextChunkHash) {
                //fmt.Println("FOUND CHUNK WITH HASH", nextChunkHash)
                // get the file chunk with this hash 
                nextChunk := bytesToHex(getChunkByIndex(file.Name, index))
                gossiper.SafeIndexedFiles.mux.Unlock()
                //fmt.Println("HASH OF CHUNK (SANITY CHECK:", bytesToHex(computeHash(hexToBytes(nextChunk))))
                return nextChunk, true, file.Metahash
            }

            index++
        }
    }

    gossiper.SafeIndexedFiles.mux.Unlock()
    return chunk, false, ""
}

// returns index of file, isMetafile, nextChunkHash, isLastChunk
func getNextChunkHashToRequest(fileOrigin string, hashValue string) (int, bool, string, bool) {
    var nextChunkHash string
    isMetafile := false
    indexOfFile := -1
    var metahash_hex string

    gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
    filesOfOrigin, exists := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin]
    gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

    if(!exists) {
        return -1, false, nextChunkHash, false
    }

    // First check if we received a metafile
    for i, fileAndIndex := range filesOfOrigin {
        metahash_hex = fileAndIndex.Metahash
        if((fileAndIndex.NextIndex == -1) && (hashValue == metahash_hex) && !fileAndIndex.Done) {
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
func getNextChunkToRequest(fileOrigin string, hashValue []byte) (int, bool, []byte) {
    var nextChunk []byte
    isMetafile := false
    indexOfFile := -1

    gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
    filesAndIndicesOfOrigin, exists := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin]
    gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

    if(!exists) {
        return -1, false, nextChunk
    }

    // First check if we received a metafile
    for i, fileAndIndex := range filesAndIndicesOfOrigin {
        if((fileAndIndex.NextIndex == -1) && bytes.Equal(hashValue, hexToBytes(fileAndIndex.Metahash))) {
            // return first chunk
            gossiper.SafeIndexedFiles.mux.Lock()
            nextChunk = getChunkByIndex(gossiper.SafeIndexedFiles.IndexedFiles[bytesToHex(hashValue)].Name, 0)
            gossiper.SafeIndexedFiles.mux.Unlock()
            isMetafile = true
            return i, isMetafile, nextChunk
        }
    }

    // Otherwise we received a chunk, must range over all metafiles and see which chunk hash 
    for i, fileAndIndex := range filesAndIndicesOfOrigin {
        gossiper.SafeIndexedFiles.mux.Unlock()
        filename := gossiper.SafeIndexedFiles.IndexedFiles[fileAndIndex.Metahash].Name
        gossiper.SafeIndexedFiles.mux.Unlock()

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

    if(err != nil) {
        fmt.Println("Error :", err, " cannot read index :", index, " of file ", filename, " n is :", n)
    }

    return chunk[0:n]

}

func computeFileIndices(filename string, gotEntireFile bool) File {
    // Should we initialize the rest of the fields with make([]byte, ...) ? 
    var f File

    // Load file
    file, err := os.Open("Peerster/_SharedFiles/" + filename)
    isError(err)
    defer file.Close()

    if(err != nil) {
        return f
    }

    f = File{Name: filename}

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
    //fmt.Println("Metafile :", f.Metafile, " or as bytes :", metafile)

    // compute metahash
    h = sha256.New()
    h.Write(metafile)
    metahash := h.Sum(nil)

    f.Metahash = bytesToHex(metahash)

    if(gotEntireFile) {
        f.NextIndex = int(getNbChunksFromMetafile(f.Metafile))
        f.Done = true
    } else {
        f.NextIndex = -1
        f.Done = false
    }

    return f
}