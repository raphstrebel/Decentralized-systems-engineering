package main

import(
	"fmt"
	"flag"
	"protobuf"
	"math/rand"
	"mux"
    "handlers"
    "net/http"
    "strings"
)

func listenUIPort() {

	defer gossiper.UIPortConn.Close()
 	
    packetBytes := make([]byte, UDP_PACKET_SIZE)
 
    for true {

        _,_,err := gossiper.UIPortConn.ReadFromUDP(packetBytes)
        isError(err)

        receivedPkt := &ClientPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

        if(receivedPkt.Message != nil) {

	        msg := receivedPkt.Message.Text

	        fmt.Println("CLIENT MESSAGE", msg) 
	        fmt.Println("PEERS", gossiper.Peers_as_single_string)

	        gossiper.SafeNextClientMessageIDs.mux.Lock()

        	rumorMessage := RumorMessage{
        		Origin: gossiper.Name, 
        		ID: gossiper.SafeNextClientMessageIDs.NextClientMessageID,
        		Text: msg,
        	}

        	gossiper.SafeNextClientMessageIDs.NextClientMessageID ++
        	gossiper.SafeNextClientMessageIDs.mux.Unlock()

        	gossiper.LastRumor = rumorMessage
        	stateID := updateStatusAndRumorArray(rumorMessage, false)

        	if(len(gossiper.Peers) > 0) {
        		go rumormongering(rumorMessage, false)
        		if(stateID != "present") {
        			fmt.Println("Error : client message", rumorMessage, "has state id :", stateID)
        		}
        	}
		} else if(receivedPkt.Private != nil) {
			privateMsg := PrivateMessage{
                Origin : gossiper.Name,
                ID : 0,
                Text : receivedPkt.Private.Text,
                Destination : receivedPkt.Private.Destination,
                HopLimit : NORMAL_HOP_LIMIT,
            }

			address := getAddressFromRoutingTable(receivedPkt.Private.Destination)

			if(address != "") {
				sendPrivateMsgToSpecificPeer(privateMsg, address)
			}
		} else if(receivedPkt.File != nil) { // New file to be indexed

			filename := receivedPkt.File.FileName
			file, isOk := computeFileIndices(filename, true)

			if(isOk && file.Size <= MAX_FILE_SIZE) {
				// add to map of indexed files
				metahash_hex := file.Metahash

				gossiper.SafeIndexedFiles.mux.Lock()
				gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = file
				gossiper.SafeIndexedFiles.mux.Unlock()

				//fmt.Println("Metahash of file indexed is :", metahash_hex)
				//fmt.Println("Metafile of file indexed is :", file.Metafile)


				// ATTENTION : CHANGES APPLIED WITH HW 3
				gossiper.SafeFilenamesToMetahash.mux.Lock()
				_,alreadyInBlockchain := gossiper.SafeFilenamesToMetahash.FilenamesToMetahash[file.Name]
				gossiper.SafeFilenamesToMetahash.mux.Unlock()

				if(!alreadyInBlockchain) {
					f := File {
						Name: file.Name,
						Size: int64(file.Size),
						MetafileHash: hexToBytes(metahash_hex),
					}

					txPublish := TxPublish{
						File: f,
						HopLimit: NORMAL_HOP_LIMIT,
					}

					gossiper.PendingTx = append(gossiper.PendingTx, txPublish)

					broadcastTxPublishToAllPeersExcept(txPublish, "")

				} else {
					fmt.Println("Error : Filename already exists :", file.Name)
				}

			} else {
				fmt.Println("Error : file inexistent or too big :", file)
			}
		} else if(receivedPkt.Request != nil) {
			request_metahash := receivedPkt.Request.Request
			dest := receivedPkt.Request.Destination
			filename := receivedPkt.Request.FileName

			address := getAddressFromRoutingTable(dest)

			gossiper.SafeIndexedFiles.mux.Lock()
			_, isIndexed := gossiper.SafeIndexedFiles.IndexedFiles[request_metahash]
			gossiper.SafeIndexedFiles.mux.Unlock()

			// Not already downloaded and we know the address
			if(!isIndexed && address != "") {

				gossiper.SafeIndexedFiles.mux.Lock()
				gossiper.SafeIndexedFiles.IndexedFiles[request_metahash] = MyFileStruct{
				    Name: filename,
				    Metafile : "",
				    Metahash: request_metahash,
				    NextIndex: -1,
				}
				gossiper.SafeIndexedFiles.mux.Unlock()

				dataRequest := DataRequest {
					Origin : gossiper.Name,
					Destination : dest,
					HopLimit : NORMAL_HOP_LIMIT,
					HashValue : hexToBytes(request_metahash),
				}

				gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
				requestedArray, alreadyRequesting := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[dest]

				if(!alreadyRequesting) {
					gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[dest] = []FileAndIndex{}
				}

				gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[dest] = append(requestedArray, FileAndIndex{
					Metahash : request_metahash,
					NextIndex : -1,
					Done : false,
				})

				gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

				// Create an empty file with name "filename" in Downloads
				createEmptyFile(filename)

				fmt.Println("DOWNLOADING metafile of", filename, "from", dest)

				sendDataRequestToSpecificPeer(dataRequest, address)
				makeDataRequestTimer(dest, dataRequest)
			}
		} else if(receivedPkt.Search != nil) {
			keywordsAsString := receivedPkt.Search.Keywords
			receivedBudget := receivedPkt.Search.Budget

			var budget uint64
			budgetGiven := true

			if(receivedBudget == 0) {
				budgetGiven = false
				budget = 2
			} else {
				budget = uint64(receivedBudget)
			}

			keywords := strings.Split(keywordsAsString, ",")

			gossiper.SafeSearchRequests.mux.Lock()
			_,exists := gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString]
			gossiper.SafeSearchRequests.mux.Unlock()

			if(!exists) {
				gossiper.SafeSearchRequests.mux.Lock()
				gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString] = SearchRequestInformation{
					Keywords: keywords,
					NbOfMatches: 0,
					BudgetIsGiven: budgetGiven,
				}
				gossiper.SafeSearchRequests.mux.Unlock()


				// Initialize the yet not seen keywords
				gossiper.SafeKeywordToInfo.mux.Lock()

				for _, keyword := range keywords {
					_, alreadyExists := gossiper.SafeKeywordToInfo.KeywordToInfo[keyword]
					
					if(!alreadyExists) {
						gossiper.SafeKeywordToInfo.KeywordToInfo[keyword] = FileChunkInfoAndNbMatches {
							FilesAndChunksInfo: []FileAndChunkInformation{},
							NbOfMatches: 0,
						}
					}
				}
				gossiper.SafeKeywordToInfo.mux.Unlock()
			}

			// Send search request to up to "budget" neighbours :
			nb_peers := uint64(len(gossiper.Peers))

			if(nb_peers > 0) {
				// send to all peers by setting a budget as evenly distributed as possible
				budgetForAll, nbPeersWithBudgetIncreased := getDistributionOfBudget(budget, nb_peers)

				total := uint64(budgetForAll*nb_peers + nbPeersWithBudgetIncreased)
					
				// Sanity check
				if(total != budget) {
					fmt.Println("ERROR : the total budget allocated and the total budget received is not the same :", total, " != ", budget)
				}

				if(budgetGiven) {
					sendSearchRequestToNeighbours(keywords, budgetForAll, nbPeersWithBudgetIncreased)
				} else {
					sendPeriodicalSearchRequest(keywordsAsString, nb_peers)
				}
			}
		} else if(receivedPkt.SearchDownload != nil) {
			filename := receivedPkt.SearchDownload.Filename
			metahash_hex := receivedPkt.SearchDownload.Metahash


			gossiper.SafeIndexedFiles.mux.Lock()
			f, exists := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
			gossiper.SafeIndexedFiles.mux.Unlock()
			

			if(!exists || !f.Done) {
				gossiper.SafeIndexedFiles.mux.Lock()

				f = MyFileStruct{
					Name: getFilename(filename),
					Metahash: metahash_hex,
				}

				gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
				gossiper.SafeIndexedFiles.mux.Unlock()

				// Now request the download the whole file with help of the map[origin]->chunk
				searchReplyOrigin := getNextChunkDestination(metahash_hex, 0, "")
				
				requestMetafileOfHashAndDest(metahash_hex, searchReplyOrigin)
				//downloadFileWithMetahash(filename, metahash_hex)
			} else {
				fmt.Println("File already downloaded")
			}
		} else {
			fmt.Println("Error : Unknown packet received from client :", receivedPkt)
		}
    }
}

func listenGossipPort() {

	defer gossiper.GossipPortConn.Close()
	packetBytes := make([]byte, UDP_PACKET_SIZE)
	var tableUpdated bool
	var stateID string

	for true {	

        _,addr,err := gossiper.GossipPortConn.ReadFromUDP(packetBytes)
        isError(err)

		receivedPkt := &GossipPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

		peerAddr := addr.String()		
		updatePeerList(peerAddr)

        if(receivedPkt.Rumor != nil) {

	        msg :=  receivedPkt.Rumor.Text
	        origin := receivedPkt.Rumor.Origin
	        id := receivedPkt.Rumor.ID

	        rumorMessage := RumorMessage{
        		Origin: origin,
        		ID: id,
        		Text: msg,
        	}

        	gossiper.LastRumor = rumorMessage
			
			// Check to see if message is not a route rumor
			if(msg != "") {		
				stateID = updateStatusAndRumorArray(rumorMessage, false)
				if(stateID == "present") {
					fmt.Println("RUMOR origin", origin, "from", peerAddr, "ID", id, "contents", msg) 
        			fmt.Println("PEERS", gossiper.Peers_as_single_string)
				}
        	} else {
        		stateID = updateStatusAndRumorArray(rumorMessage, true)
        	}

        	tableUpdated = false

        	if(stateID == "present" || stateID == "future") {
	        	tableUpdated = updateRoutingTable(rumorMessage, peerAddr)
	        }

        	if(tableUpdated) {
        		fmt.Println("DSDV", origin, peerAddr)
        	}

        	sendStatusMsgToSpecificPeer(peerAddr)
        	
        	if(len(gossiper.Peers) > 0) {
        		if(stateID == "present") {
        			go rumormongering(rumorMessage, false)
        		} else {
	        		if(rand.Int() % 2 == 0) {
	        			go rumormongering(rumorMessage, true)
	        		}
	        	}
        	}
	    } else if(receivedPkt.Status != nil) {

	    	isResponse := isStatusResponse(peerAddr)
	    	
	    	if(isResponse) {
	    		removeFinishedTimer(peerAddr)
	    	}

			peerStatus :=  receivedPkt.Status.Want

			printStatusReceived(peerStatus, peerAddr)
			fmt.Println("PEERS", gossiper.Peers_as_single_string)

			rumorToSend, statusToSend := compareStatus(peerStatus, peerAddr)

			if(rumorToSend != nil) {

				sendRumorMsgToSpecificPeer(*rumorToSend, peerAddr)
				makeTimer(peerAddr, *rumorToSend)

			} else if(statusToSend != nil) {

				sendStatusMsgToSpecificPeer(peerAddr)

			} else {
				fmt.Println("IN SYNC WITH", peerAddr)

				if(rand.Int() % 2 == 0) {
					// Reading rumor message map ?
					if(len(gossiper.SafeRumors.RumorMessages) > 0) {
		        		go rumormongering(gossiper.LastRumor, true)
		        	}
	        	}
			}
		} else if(receivedPkt.Private != nil) {
			origin := receivedPkt.Private.Origin
			dest := receivedPkt.Private.Destination
			hopLimit := receivedPkt.Private.HopLimit
			msg := receivedPkt.Private.Text

			privateMsg := PrivateMessage{
	                Origin : origin,
	                ID : receivedPkt.Private.ID,
	                Text : msg,
	                Destination : dest,
	                HopLimit : hopLimit,
            	}

			if(dest == gossiper.Name) {
				fmt.Println("PRIVATE origin", origin, "hop-limit", hopLimit, "contents", msg)
				gossiper.PrivateMessages = append(gossiper.PrivateMessages, privateMsg)
			} else {
            	address := getAddressFromRoutingTable(dest)

            	if(address != "" && hopLimit > 0) {
					privateMsg.HopLimit--
					sendPrivateMsgToSpecificPeer(privateMsg, address)
				}
			}
		} else if(receivedPkt.DataRequest != nil) {

			requestOrigin := receivedPkt.DataRequest.Origin
			dest := receivedPkt.DataRequest.Destination
			hopLimit := receivedPkt.DataRequest.HopLimit
			hashValue_bytes := receivedPkt.DataRequest.HashValue
			hashValue_hex := bytesToHex(hashValue_bytes)

			if(dest == gossiper.Name) {
				address := getAddressFromRoutingTable(requestOrigin)

				if(address != "") {
					gossiper.SafeIndexedFiles.mux.Lock()
					file, isMetaHash := gossiper.SafeIndexedFiles.IndexedFiles[hashValue_hex]
					gossiper.SafeIndexedFiles.mux.Unlock()
					
					// check if hashValue is a metahash, if yes send the metafile
					if(isMetaHash) {
						dataReply := DataReply{
							Origin : gossiper.Name,
							Destination : requestOrigin,
							HopLimit : NORMAL_HOP_LIMIT,
							HashValue : hexToBytes(file.Metahash),
							Data : hexToBytes(file.Metafile),
						}

						sendDataReplyToSpecificPeer(dataReply, address)

					} else {
						nextChunkHash := hashValue_hex
						
						chunkToSend, fileIsIndexed, _ := checkFilesForNextChunk(requestOrigin, nextChunkHash)

						var dataReply DataReply

						if(fileIsIndexed) {
							dataReply = DataReply{
								Origin : gossiper.Name,
								Destination : requestOrigin,
								HopLimit : NORMAL_HOP_LIMIT,
								HashValue : hexToBytes(hashValue_hex), // hash of i'th chunk
								Data : hexToBytes(chunkToSend), // get i'th chunk
							}
						} else {

							dataReply = DataReply{
								Origin : gossiper.Name,
								Destination : requestOrigin,
								HopLimit : NORMAL_HOP_LIMIT,
								HashValue : hashValue_bytes, 
								Data : []byte{}, // send empty data to show that we do not posses the file 
							}
						}

						sendDataReplyToSpecificPeer(dataReply, address)
					}
				}
			} else {
				address := getAddressFromRoutingTable(dest)

            	if(address != "" && hopLimit > 0) {
            		dataRequest := DataRequest {
						Origin : requestOrigin,
						Destination : dest,
						HopLimit : hopLimit - 1,
						HashValue : hashValue_bytes,
					}

					sendDataRequestToSpecificPeer(dataRequest, address)
				}
			}

		} else if(receivedPkt.DataReply != nil) {

			fileOrigin := receivedPkt.DataReply.Origin
			dest := receivedPkt.DataReply.Destination
			hashValue := receivedPkt.DataReply.HashValue
			hopLimit := receivedPkt.DataReply.HopLimit
			data := receivedPkt.DataReply.Data

			hashValue_hex := bytesToHex(hashValue)
			data_hex := bytesToHex(data)

			if(dest == gossiper.Name) {

				// check if we are expecting a reply from "fileOrigin" and cancel timer 

				isResponse := isDataResponse(fileOrigin)
	    	
		    	if(isResponse) {
		    		removeFinishedDataRequestTimer(fileOrigin)
		    	}

		    	// check that hashValue is hash of data and we are expecting a response
		    	hash := computeHash(data_hex)

		    	if(hash == hashValue_hex && isResponse && len(data_hex) != 0) {

		    		indexOfFile, isMetafile, nextChunkHashNormalProcedure, isLastChunk := getNextChunkHashToRequest(fileOrigin, hashValue_hex)

		    		gossiper.SafeAwaitingRequestsMetahash.mux.Lock()
					metafile_hex_stored, searchExistsAndHashIsMetafile := gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash[hashValue_hex]
					gossiper.SafeAwaitingRequestsMetahash.mux.Unlock()

					var metahash_hex string
					var metafile_hex string
					var nextChunkIndex uint64
					var nextChunkHash string
					var downloadIsDone bool

					if(!searchExistsAndHashIsMetafile) {
						metahash_hex, metafile_hex, nextChunkHash, nextChunkIndex, downloadIsDone = getMetahashMetafileIndexOfNextChunkFromAwaitingRequests(hashValue_hex)
					}
					
					// Check if we are waiting for a metafile for a search request (if yes we don't need to download all the file)
					if(searchExistsAndHashIsMetafile) {

						if(metafile_hex_stored == "") {
							metafile_hex = data_hex
							metahash_hex = hashValue_hex

							gossiper.SafeAwaitingRequestsMetahash.mux.Lock()
							gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash[metahash_hex] = metafile_hex
							gossiper.SafeAwaitingRequestsMetahash.mux.Unlock()

							// add the metafile to our indexed files 
							gossiper.SafeIndexedFiles.mux.Lock()
			    			f := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
			    			f.Metafile = metafile_hex
			    			f.NextIndex = 0
			    			gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
							gossiper.SafeIndexedFiles.mux.Unlock()
							

							gossiper.SafeKeywordToInfo.mux.Lock()
							allKeywordsToInfo := gossiper.SafeKeywordToInfo.KeywordToInfo

							var changed bool

							// Loop over all keywords :
							for keyword, fileChunkInfoAndNbMatches := range allKeywordsToInfo {

								newFileChunkInfoAndNbMatches := fileChunkInfoAndNbMatches
								
								// Check all infos of files of this keyword:
								for fileIndex := range fileChunkInfoAndNbMatches.FilesAndChunksInfo {

									fileInfo := fileChunkInfoAndNbMatches.FilesAndChunksInfo[fileIndex]

									if(fileInfo.Metahash == metahash_hex && fileInfo.Metafile == "") {

										fileInfo.Metafile = metafile_hex
										fileInfo.NbOfChunks = getNbChunksFromMetafile(metafile_hex)

										// Check if we know where all the chunks are
										nbChunksWeHave := getNbTotalChunksOfMap(fileInfo.ChunkOriginToIndices)

										if(nbChunksWeHave == fileInfo.NbOfChunks) {
											fileInfo.FoundAllChunks = true
											changed = true
										}

										newFileChunkInfoAndNbMatches.FilesAndChunksInfo[fileIndex] = fileInfo
									}
								}

								gossiper.SafeKeywordToInfo.KeywordToInfo[keyword] = newFileChunkInfoAndNbMatches

								if(changed) {
									sendMatchToFrontend(keyword, false, fileOrigin)
								}
							}

							gossiper.SafeKeywordToInfo.mux.Unlock()

							downloadFileWithMetahash(f.Name, metahash_hex)

						} else { 
							fmt.Println("Error : Got metafile but not in AwaitingRequestsMetahash :", gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash)
						}	
					} else if(metahash_hex != "") {
						// we already have the metafile and we are downloading the rest, we received the next chunk
						// loop over all metafiles of all files being requested :
							
						//currentHashValue_hex := bytesToHex(data)
						//metahash_hex, metafile_hex, nextChunkHash, nextChunkIndex, downloadIsDone := getMetahashMetafileIndexOfNextChunkFromAwaitingRequests(currentHashValue_hex)

						gossiper.SafeIndexedFiles.mux.Lock()
		    			f,_ := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
		    			f.Metafile = metafile_hex
		    			f.NextIndex++
		    			nextChunkIndex = uint64(f.NextIndex)
			    		gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
						gossiper.SafeIndexedFiles.mux.Unlock()
	
						writeChunkToFile(f.Name, data_hex)

						if(len(metafile_hex) == (f.NextIndex)*CHUNK_HASH_SIZE_IN_HEXA) {
							downloadIsDone = true
						} else {
							nextChunkHash = metafile_hex[f.NextIndex*CHUNK_HASH_SIZE_IN_HEXA:(f.NextIndex+1)*CHUNK_HASH_SIZE_IN_HEXA]
						}

						if(downloadIsDone) {
							fmt.Println("RECONSTRUCTED file", f.Name)

							// copy file to _Downloads
							copyFileToDownloads(f.Name)

							gossiper.SafeIndexedFiles.mux.Lock()
							f.Done = true
							gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
							gossiper.SafeIndexedFiles.mux.Unlock()

							gossiper.SafeAwaitingRequestsMetahash.mux.Lock()
							delete(gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash, metahash_hex)
							gossiper.SafeAwaitingRequestsMetahash.mux.Unlock()
						} else {
							// request next chunk
							newDestination := getNextChunkDestination(metahash_hex, nextChunkIndex, fileOrigin)
							if(newDestination != "") {
								newAddress := getAddressFromRoutingTable(newDestination)
									
								fmt.Println("DOWNLOADING", f.Name, "chunk", nextChunkIndex+1, "from", newDestination)
									
								// Send request for next chunk
								dataRequest := DataRequest {
									Origin : gossiper.Name,
									Destination : newDestination,
									HopLimit : NORMAL_HOP_LIMIT,
									HashValue : hexToBytes(nextChunkHash),
								}
								
								sendDataRequestToSpecificPeer(dataRequest, newAddress)
								makeDataRequestTimer(newDestination, dataRequest)
							} 
						}
					} else if(indexOfFile >= 0) {	
		    			address := getAddressFromRoutingTable(fileOrigin)

		    			if(isMetafile) {

				    		metahash_hex = hashValue_hex
				    		metafile_hex := data_hex

			    			if(len(metafile_hex) % 64 == 0) {
			    				// add fileOrigin and file to RequestDestinationToFileAndIndex
			    				gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
				    			gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex = 0
				    			gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metahash = metahash_hex
				    			gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metafile = metafile_hex
				    			chunkIndex := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex + 1
				    			gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

				    			gossiper.SafeIndexedFiles.mux.Lock()
				    			f := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
				    			f.Metafile = metafile_hex
				    			f.NextIndex = 0
				    			gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
				    			gossiper.SafeIndexedFiles.mux.Unlock()


					    		fmt.Println("DOWNLOADING", f.Name, "chunk", chunkIndex, "from", fileOrigin)

					    		// Request first chunk
					    		firstChunk := metafile_hex[0:CHUNK_HASH_SIZE_IN_HEXA]

					   			dataRequest := DataRequest {
									Origin : gossiper.Name,
									Destination : fileOrigin,
									HopLimit : NORMAL_HOP_LIMIT,
									HashValue : hexToBytes(firstChunk),
								}
									
								sendDataRequestToSpecificPeer(dataRequest, address)
								makeDataRequestTimer(fileOrigin, dataRequest)

			    			} else {
			    				fmt.Println("Wrong metafile (not of chunks of size CHUNK_SIZE) : ", metafile_hex)
			    			}
						} else if(isLastChunk) {
							// Save the data received to the file name :
							gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
							metahash_hex = gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metahash
							//chunkIndex := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex + 1
							gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

							gossiper.SafeIndexedFiles.mux.Lock()
							f := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]

							writeChunkToFile(f.Name, data_hex)

							f.NextIndex++
							gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
							gossiper.SafeIndexedFiles.mux.Unlock()

							// check if the hash of metafile is metahash
							if(hashValue_hex == f.Metafile[len(f.Metafile)-2*HASH_SIZE:len(f.Metafile)]) {
								fmt.Println("RECONSTRUCTED file", f.Name)
								// copy the file to _Downloads
								copyFileToDownloads(f.Name)

								// erase the file from RequestDestinationToFileAndIndex[fileOrigin]
								gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
								gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Done = true
								gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

								gossiper.SafeIndexedFiles.mux.Lock()
								f.Done = true
								gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
								gossiper.SafeIndexedFiles.mux.Unlock()


							} else {
								fmt.Println("Error : file was not reconstructed :", f)
							}
						} else {

							// Save the data received to the file name :
							gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
							metahash_hex = gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metahash
							gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex++
							chunkIndex := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex+1
							gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

							gossiper.SafeIndexedFiles.mux.Lock()
							f := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
							f.NextIndex++
							gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
							gossiper.SafeIndexedFiles.mux.Unlock()

							writeChunkToFile(f.Name, data_hex)

							fmt.Println("DOWNLOADING", f.Name, "chunk", chunkIndex, "from", fileOrigin)
							
							// Send request for next chunk
							dataRequest := DataRequest {
								Origin : gossiper.Name,
								Destination : fileOrigin,
								HopLimit : NORMAL_HOP_LIMIT,
								HashValue : hexToBytes(nextChunkHashNormalProcedure),
							}
								
							sendDataRequestToSpecificPeer(dataRequest, address)
							makeDataRequestTimer(fileOrigin, dataRequest)

						}

		    		} else {
		    			fmt.Println("ERROR : CHUNK OF FILE NOT FOUND")
		    		}
		    	} else {
		    		fmt.Println("Error : hash of data is not hashValue of dataReply", hash, " != ", hashValue_hex, 
		    			" or we do not expect response ? is response = ", isResponse)

		    		hashValue_hex := bytesToHex(hashValue)

		    		gossiper.SafeIndexedFiles.mux.Lock()
					_, exists := gossiper.SafeIndexedFiles.IndexedFiles[hashValue_hex]

					if(exists) {
						// we delete the file
						delete(gossiper.SafeIndexedFiles.IndexedFiles, hashValue_hex)
					}
					gossiper.SafeIndexedFiles.mux.Unlock()
		    	}
			} else {
				// Send to next in routing table 
				address := getAddressFromRoutingTable(dest)

            	if(address != "" && hopLimit != 0) {
            		dataReply := DataReply {
						Origin : fileOrigin,
						Destination : dest,
						HopLimit : hopLimit - 1,
						HashValue : hashValue,
						Data : hexToBytes(data_hex),
					}

					sendDataReplyToSpecificPeer(dataReply, address)
				}
			}
		} else if(receivedPkt.SearchRequest != nil) {
			searchOrigin := receivedPkt.SearchRequest.Origin
			budget := receivedPkt.SearchRequest.Budget
			keywords := receivedPkt.SearchRequest.Keywords
			keywordsAsString := getKeywordsAsString(keywords)

			// We do this to keep track of requests in the last 0.5 seconds
			originAndKeyword := OriginAndKeywordsStruct {
				Origin: searchOrigin,
				Keywords: keywordsAsString,
			}

			gossiper.SafeOriginAndKeywords.mux.Lock()
			_, alreadyExists := gossiper.SafeOriginAndKeywords.OriginAndKeywords[originAndKeyword]
			gossiper.SafeOriginAndKeywords.OriginAndKeywords[originAndKeyword] = true
			gossiper.SafeOriginAndKeywords.mux.Unlock()

			address := getAddressFromRoutingTable(searchOrigin)

			if(address != "" && budget > 0 && !alreadyExists) {

				// set timer for the request
				setSearchRequestTimer(originAndKeyword)

				searchReply := SearchReply{
					Origin: gossiper.Name,
					Destination: searchOrigin,
					HopLimit: NORMAL_HOP_LIMIT,
					Results: []*SearchResult{},
				}

				var newResult *SearchResult
				var matchingFiles []MyFileStruct
				var chunkMap []uint64

				// Check all indexed file names for keywords
				for _,keyword := range keywords {
					// for every keywords, check if any file in SafeIndexedFiles has a name that matches the search
					matchingFiles = getFilesWithMatchingFilenames(keyword)

					// if any file matches the search, make a new search result to add to the search reply
					for _,file := range matchingFiles {
						
						chunkMap = getChunkMap(file)

						newResult = &SearchResult {
							FileName: file.Name,
							MetafileHash: hexToBytes(file.Metahash),
							ChunkMap: chunkMap,
							ChunkCount: getNbChunksFromMetafile(file.Metafile),
						}

						searchReply.Results = append(searchReply.Results, newResult)
					}
				}

				sendSearchReplyToSpecificPeer(searchReply, address)

				// decrease budget, then send to all peers (except the origin of the search) by setting a budget as evenly distributed as possible
				budget--
				nb_peers := len(gossiper.Peers)

				if(budget > 0 && nb_peers > 1) {
					budgetForAll, nbPeersWithBudgetIncreased := getDistributionOfBudget(budget, uint64(nb_peers-1))
					if(budgetForAll > 0) {
						propagateSearchRequest(keywords, budgetForAll, nbPeersWithBudgetIncreased, searchOrigin, address)
					}
				}
			} else {
				fmt.Println("Either address is not found :", address, ", budget is 0 :", budget, " or the search was already done in the last 0.5sec :", alreadyExists)
			}
		} else if(receivedPkt.SearchReply != nil) {

			searchReplyOrigin := receivedPkt.SearchReply.Origin
			dest := receivedPkt.SearchReply.Destination
			hopLimit := receivedPkt.SearchReply.HopLimit
			results := receivedPkt.SearchReply.Results

			if(dest == gossiper.Name) {
				// check out the replies we got
				keywordsMatchedMap := make(map[string]bool)

				gossiper.SafeKeywordToInfo.mux.Lock()

				// check all files received as result to see if they match one or more keywords and add them to the files of the keyword they match 
				for _,file := range results {
					metahash_hex := bytesToHex(file.MetafileHash)

					// check if we already have the file :
					gossiper.SafeIndexedFiles.mux.Lock()
					f := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
					gossiper.SafeIndexedFiles.mux.Unlock()

					chunkMap := getNormalChunkMap(file.ChunkMap)

					// We do not have the file in our _Downloads folder 
					if(!f.Done) {
						// range over key,value of map KeywordToInfo
						for keyword,_ := range gossiper.SafeKeywordToInfo.KeywordToInfo {

							fileChunkInfoAndNbMatchesOfKeyword := gossiper.SafeKeywordToInfo.KeywordToInfo[keyword]
							fileAndChunkInfoOfKeyword := fileChunkInfoAndNbMatchesOfKeyword.FilesAndChunksInfo
							keywordMatchesFilename := matchesRegex(file.FileName, keyword)
							
							if(keywordMatchesFilename) {
								keywordsMatchedMap[keyword] = true

								alreadyContains, indexOfFile := containsFileMetahash(fileAndChunkInfoOfKeyword, metahash_hex)

								if(!alreadyContains) {
									// Must append this file to our list of files for this keyword : (we cannot increment the matches as we do not have the metafile)
									newFileAndChunkInfo := FileAndChunkInformation{
										Filename: file.FileName,
										Metahash: metahash_hex,
										NbOfChunks: file.ChunkCount,
										FoundAllChunks: false,
										AlreadyShown: false,
										ChunkOriginToIndices: make(map[string][]uint64),
									}
									newFileAndChunkInfo.ChunkOriginToIndices[searchReplyOrigin] = chunkMap//file.ChunkMap

									// Check if we have the metafile in our indexed files :
									indexedFile, isContained := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]

									if(!isContained || indexedFile.Metafile == "") {
										// Request the metafile
										//fmt.Println("Files of keywords does not contain file, we do not have the metafile, requesting metafile for :", metahash_hex)
										// fmt.Println("DOWNLOADING METAFILE ...")

										chunksAvailableAtOriginOfResult := getNbTotalChunksOfMap(newFileAndChunkInfo.ChunkOriginToIndices)

										if(chunksAvailableAtOriginOfResult == newFileAndChunkInfo.NbOfChunks) {
											newFileAndChunkInfo.FoundAllChunks = true

											// In this scenario when receiving the same file twice we increment nb of matches twice...
											// TODO check if next line is needed
											fileChunkInfoAndNbMatchesOfKeyword.NbOfMatches++
											gossiper.SafeKeywordToInfo.KeywordToInfo[keyword] = fileChunkInfoAndNbMatchesOfKeyword
										}
									} else {
										newFileAndChunkInfo.Metafile = indexedFile.Metafile
										//newFileAndChunkInfo.NbOfChunks = getNbChunksFromMetafile(indexedFile.Metafile)

										if(indexedFile.Done) {
											newFileAndChunkInfo.FoundAllChunks = true
											newFileAndChunkInfo.AlreadyShown = true
										} 
									}

									fileAndChunkInfoOfKeyword = append(fileAndChunkInfoOfKeyword, newFileAndChunkInfo)
									
									fileChunkInfoAndNbMatchesOfKeyword.FilesAndChunksInfo = fileAndChunkInfoOfKeyword
									fileChunkInfoAndNbMatchesOfKeyword.NbOfMatches = 0
									gossiper.SafeKeywordToInfo.KeywordToInfo[keyword] = fileChunkInfoAndNbMatchesOfKeyword

								} else {
									// Our keyword already has the file, we have the index of the file in the array 
									
									// so we just need to update the chunkIndices with file.ChunkMap
									fileAndChunkInfoToUpdate := fileAndChunkInfoOfKeyword[indexOfFile]

									// sanity check :
									if(fileAndChunkInfoToUpdate.Metahash != metahash_hex) {
										fmt.Println("ERROR : containsFileMetahash method outputted wrong index :", indexOfFile, "for array :", fileAndChunkInfoOfKeyword, "and resulting file :", file)
									} 

									// loop over all values (chunk indices) of the search result array, add the missing ones in the chunkOriginToIndex map :
									for _, resultChunkIndex := range chunkMap {//file.ChunkMap {
										if(!containsUint64(fileAndChunkInfoToUpdate.ChunkOriginToIndices, resultChunkIndex)) {
											_, originExists := fileAndChunkInfoToUpdate.ChunkOriginToIndices[searchReplyOrigin]

											if(originExists) {
												fileAndChunkInfoToUpdate.ChunkOriginToIndices[searchReplyOrigin] = append(fileAndChunkInfoToUpdate.ChunkOriginToIndices[searchReplyOrigin], resultChunkIndex)
											} else {
												fileAndChunkInfoToUpdate.ChunkOriginToIndices[searchReplyOrigin] = []uint64{resultChunkIndex}
											}
										}
									}

									// check if we have the metafile
									if(fileAndChunkInfoToUpdate.Metafile != "") {

										// check if we have all chunks
										nbTotalChunks := getNbTotalChunksOfMap(fileAndChunkInfoToUpdate.ChunkOriginToIndices)

										if(nbTotalChunks == fileAndChunkInfoToUpdate.NbOfChunks && !fileAndChunkInfoToUpdate.FoundAllChunks) {
											fileAndChunkInfoToUpdate.FoundAllChunks = true
											fileAndChunkInfoOfKeyword[indexOfFile] = fileAndChunkInfoToUpdate

											fileChunkInfoAndNbMatchesOfKeyword.NbOfMatches++

											fileChunkInfoAndNbMatchesOfKeyword.FilesAndChunksInfo = fileAndChunkInfoOfKeyword
											gossiper.SafeKeywordToInfo.KeywordToInfo[keyword] = fileChunkInfoAndNbMatchesOfKeyword
										}
									}
								}
							}
						}
					}
				}
				gossiper.SafeKeywordToInfo.mux.Unlock()  

				updateSearchRequestsByKeywords(keywordsMatchedMap, searchReplyOrigin)

			} else { 
				// we are not the destination, send message to destination
				address := getAddressFromRoutingTable(dest)

	            if(address != "" && hopLimit > 0) {
	           		searchReply := SearchReply {
						Origin : searchReplyOrigin,
						Destination : dest,
						HopLimit : hopLimit - 1,
						Results : results,
					}

					sendSearchReplyToSpecificPeer(searchReply, address)
				}
			}
		} else if(receivedPkt.TxPublish != nil) {

			txPublish := receivedPkt.TxPublish
			file := receivedPkt.TxPublish.File
			hopLimit := receivedPkt.TxPublish.HopLimit

			filename := file.Name

			gossiper.SafeFilenamesToMetahash.mux.Lock()
			_, filenameExists := gossiper.SafeFilenamesToMetahash.FilenamesToMetahash[filename]
			gossiper.SafeFilenamesToMetahash.mux.Unlock()

			if(!filenameExists) {

				if(hopLimit > 0) {
					txPublish.HopLimit = hopLimit - 1
					//fmt.Println("Broadcasting tx publish to:", peerAddr)
					broadcastTxPublishToAllPeersExcept(*txPublish, peerAddr)
				}
				
				gossiper.PendingTx = append(gossiper.PendingTx, *txPublish)

			} else {
				fmt.Println("Error : Filename already exists :", file.Name)
			}
		} else if(receivedPkt.BlockPublish != nil) {
			
			//fmt.Println("Received blockPublish from :", peerAddr)

			blockPublish := receivedPkt.BlockPublish

			handleNewBlockArrival(*blockPublish, peerAddr)

		} else {
			fmt.Println("ERROR : Packet of unknown type :", receivedPkt)
		}
    }
}

func handleNewBlockArrival(blockPublish BlockPublish, peerAddr string) {

	block := blockPublish.Block
	hopLimit := blockPublish.HopLimit

	// check if PoW is valid
	isValid, blockHash_hex := checkBlockPoW(block)

	gossiper.SafeFilenamesToMetahash.mux.Lock()
	allFilenamesFree := checkAllFilenamesAreFree(block)
	gossiper.SafeFilenamesToMetahash.mux.Unlock()

	//if(isValid && allFilenamesFree && transactionsNotEmpty) {
	if(isValid && allFilenamesFree) {


		// check if we already have seen the block
		gossiper.SafeBlockchain.mux.Lock()
		_, contains := gossiper.SafeBlockchain.Blockchain[blockHash_hex]
		gossiper.SafeBlockchain.mux.Unlock()

		// if we do not already have this block in our blockchain
		if(!contains) {

			// check if we have seen the parent block
			parentBlockHash := make([]byte, 32)
			copy(parentBlockHash, block.PrevHash[:])
			parentBlockHash_hex := bytesToHex(parentBlockHash)

			gossiper.SafeBlockchain.mux.Lock()
			_, containsParent := gossiper.SafeBlockchain.Blockchain[parentBlockHash_hex]
			gossiper.SafeBlockchain.mux.Unlock()

			if(containsParent) {
				//fmt.Println("contains parent")

				// add block to the blockchain
				gossiper.SafeBlockchain.mux.Lock()
				gossiper.SafeBlockchain.Blockchain[blockHash_hex] = block
				gossiper.SafeBlockchain.mux.Unlock()

				// check if parentBlock is a head :
				gossiper.SafeHeadsToLength.mux.Lock()
				parentLength, parentIsHead := gossiper.SafeHeadsToLength.HeadToLength[parentBlockHash_hex]
				gossiper.SafeHeadsToLength.mux.Unlock()

				if(parentIsHead) {


					// Replace head by new block hash and increment length of the chain

					newForkLength := parentLength+1

					if(parentBlockHash_hex != GENESIS_BLOCK) {
						gossiper.SafeHeadsToLength.mux.Lock()
						delete(gossiper.SafeHeadsToLength.HeadToLength, parentBlockHash_hex)
						gossiper.SafeHeadsToLength.HeadToLength[blockHash_hex] = newForkLength
						gossiper.SafeHeadsToLength.mux.Unlock()
					} else {
						gossiper.SafeHeadsToLength.mux.Lock()
						gossiper.SafeHeadsToLength.HeadToLength[blockHash_hex] = newForkLength
						gossiper.SafeHeadsToLength.mux.Unlock()
					}

					// check if the previous block was the head of the longest chain
					if(gossiper.LongestChainHead == parentBlockHash_hex) {
						// replace the longest chain's head with the new block and add all transactions to filename->metahash map
						gossiper.LongestChainHead = blockHash_hex

						if(len(block.Transactions) > 0) {
							gossiper.SafeFilenamesToMetahash.mux.Lock()
							addTransactionsToFilenameMetahashMap(block.Transactions)
							gossiper.SafeFilenamesToMetahash.mux.Unlock()
						}

						printLongestChain()
					} else {

						// the parent block is the head of a fork, and our block is the new head of that fork
						gossiper.SafeHeadsToLength.mux.Lock()
						mainChainLength := gossiper.SafeHeadsToLength.HeadToLength[gossiper.LongestChainHead]
						gossiper.SafeHeadsToLength.mux.Unlock()

						// check if the length of the fork chain is now greater than the longest chain
						if(mainChainLength < newForkLength) {

							// We must switch our main chain to the fork

							// delete all transactions on last longest chain until we reach the intersection of main and fork block

							gossiper.SafeBlockchain.mux.Lock()
							forkMainIntersection_hex := getIntersectionBetweenMainAndFork(blockHash_hex)
							gossiper.SafeBlockchain.mux.Unlock()

							gossiper.SafeFilenamesToMetahash.mux.Lock()
							nbRewinds := deleteTransactionsOfLongestChainFromBlockToBlock(gossiper.LongestChainHead, forkMainIntersection_hex, 0)

							// add all transactions of new longest chain
							addTransactionsOfForkFromBlockToBlock(blockHash_hex, forkMainIntersection_hex)
							gossiper.SafeFilenamesToMetahash.mux.Unlock()


							// replace the longest chain's head with the new block
							gossiper.LongestChainHead = blockHash_hex

							fmt.Println("FORK-LONGER rewind", nbRewinds,"blocks")
							gossiper.SafeBlockchain.mux.Lock()
							printLongestChain()
							gossiper.SafeBlockchain.mux.Unlock()

						}
					}

				} else {
					// make fork with this block as head

					// Replace head by new block and increment length of the chain

					gossiper.SafeHeadsToLength.mux.Lock()
					gossiper.SafeHeadsToLength.HeadToLength[blockHash_hex] = getChainLengthFromBlock(parentBlockHash_hex) + 1
					gossiper.SafeHeadsToLength.mux.Unlock()

					fmt.Println("FORK-SHORTER", blockHash_hex)

				}
			} else {
				// We do not have the parent block in our blockchain

				// Check if the blockchain is empty, it yes set this block to be the first block :
				gossiper.SafeBlockchain.mux.Lock()
				if(len(gossiper.SafeBlockchain.Blockchain) == 0) {

					// Sanity check 
					if(gossiper.LongestChainHead != "") {
						fmt.Println("Error : got head :", gossiper.LongestChainHead, "but no blockchain :", gossiper.SafeBlockchain.Blockchain)
					}

					// This is the first block, so set the head of the longest blockchain, the heads of blockchains and the blockchain
					gossiper.LongestChainHead = blockHash_hex

					gossiper.SafeBlockchain.Blockchain[blockHash_hex] = block

					gossiper.SafeHeadsToLength.mux.Lock()
					gossiper.SafeHeadsToLength.HeadToLength[blockHash_hex] = 1
					gossiper.SafeHeadsToLength.mux.Unlock()

					// Add transactions to the filename to metahash map
					gossiper.SafeFilenamesToMetahash.mux.Lock()
					addTransactionsToFilenameMetahashMap(block.Transactions)
					gossiper.SafeFilenamesToMetahash.mux.Unlock()

					printLongestChain()
				}
				gossiper.SafeBlockchain.mux.Unlock()
			}


			// forward block
			if(hopLimit > 0) {
				blockPublish.HopLimit = hopLimit - 1
				broadcastBlockPublishToAllPeersExcept(blockPublish, peerAddr)
			}
		} else {
			// We have already seen the block
		}

		// todelete
		//fmt.Println()
		//fmt.Println("Printing entire chain :")
		//printEntireChain()
	} else {
		fmt.Println("Block is not valid :", blockHash_hex, "or some filenames are taken ?", !allFilenamesFree)//, "or transactions array is empty ?", !transactionsNotEmpty)

		if(blockHash_hex[0:4] != "0000") {
			fmt.Println("Error : the received block has hash :", blockHash_hex, "the block :", block)
		}
	}
}

func main() {

	UIPort := flag.String("UIPort", "8080", "a string")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "a string")
	name := flag.String("name", "", "a string")
	peers := flag.String("peers", "", "a list of strings")
	rtimer := flag.Int("rtimer", 0, "an int")
	simple := flag.Bool("simple", false, "a bool")
	flag.Parse()

	gossiper = NewGossiper("127.0.0.1:" + *UIPort, *gossipAddr, *name, *peers)
	gossiper.SafeHeadsToLength.HeadToLength[GENESIS_BLOCK] = 0

	if(*simple == true){
		// goroutines to listen on both ports simultaneously
		go simpleListenUIPort()
		simpleListenGossipPort()
	} else { 
		// goroutines to listen on both ports, run anti entropy and serve frontend simultaneously
		if(*rtimer != 0) {
			go generatePeriodicalRouteMessage(*rtimer)
		}

		go listenUIPort()
		go listenGossipPort()
		go miningProcedure()
		antiEntropy()

		r := mux.NewRouter()

		r.HandleFunc("/id", func(w http.ResponseWriter, r *http.Request) {
		    IDHandler(w, r)
		})
		r.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		    MessageHandler(w, r)
		})
		r.HandleFunc("/node", func(w http.ResponseWriter, r *http.Request) {
		    NodeHandler(w, r)
		})
		r.HandleFunc("/route", func(w http.ResponseWriter, r *http.Request) {
		    CloseNodeHandler(w, r)
		})
		r.HandleFunc("/privateMessage", func(w http.ResponseWriter, r *http.Request) {
		    PrivateMessageHandler(w, r)
		})
		r.HandleFunc("/fileSharing", func(w http.ResponseWriter, r *http.Request) {
		    FileSharingHandler(w, r)
		})
		r.HandleFunc("/fileSearching", func(w http.ResponseWriter, r *http.Request) {
		    FileSearchHandler(w, r)
		})

	    http.ListenAndServe(":8080", handlers.CORS()(r))
	}

	/* LIST OF THINGS TO DO IN ORDER OF PRIORITY :

	1. When should we forward blocks and txPublish? (if we have already seen it should we still forward it?)
	2. In what order should we print "rewind" and "longest chain"?
	3. Might have a problem with block hashes
	4. Check sizes of files, abnormal behaviour

	ATTENTION : SHOULD WE PRINT "FOUND BLOCK" EVEN WHEN MINING EMPTY BLOCK? SHOULD WE APPEND A BLOCK IF IT HAS NO TRANSACTIONS IN IT?
		
	ISSUES :
	
	change "protobuf" to "github/protobuf"
	*/

}