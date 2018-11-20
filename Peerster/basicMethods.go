package main

import(
	"fmt"
	"reflect"
	"time"
	//"math/rand"
	"net"
	"strings"
	"encoding/hex"
)

func isError(err error) {
    if err  != nil {
        fmt.Println("An error occurred : " , err)
    }
}

func contains(array []string, s string) bool {
	for _, elem := range array {
		if s == elem {
			return true
		}
	}
	return false
}

func getIndexOfFileAndIndex(array []FileAndIndex, metahash string) int {
	for i, elem := range array {
		if metahash == elem.Metahash {
			return i
		}
	}
	return -1
}

func hexToBytes(h string) []byte {
	b, err := hex.DecodeString(h)
    isError(err)
	return b
}

func bytesToHex(b []byte) string {
	return hex.EncodeToString(b)
}

// Function to update the list of known peers of our gossiper
func updatePeerList(peerAddr string) {
	if(!contains(gossiper.Peers, peerAddr)) {
		gossiper.Peers = append(gossiper.Peers, peerAddr)
		if(len(gossiper.Peers_as_single_string) != 0) {
			gossiper.Peers_as_single_string = gossiper.Peers_as_single_string + ","
	    }
	    gossiper.Peers_as_single_string = gossiper.Peers_as_single_string + peerAddr
    }
}

/* 
	This method updates our rumor array by appending the rumor if it is unseen.
	This method also updates in our status packet the "next id" of the peer that we received this rumor from.
	Returns a string : "past", "present", "future"
*/
func updateStatusAndRumorArray(rumor RumorMessage, isRouteMessage bool) string {

	// This is the first rumor we receive
	if(gossiper.StatusPacket == nil) {
		if(rumor.ID == 1){
			gossiper.SafeRumors.mux.Lock()
			// add rumor to rumors from this origin
			gossiper.SafeRumors.RumorMessages[rumor.Origin] = []RumorMessage{rumor}
			if(!isRouteMessage && rumor.Origin != gossiper.Name) {
				gossiper.RumorMessages = []RumorMessage{rumor}
			}
			gossiper.StatusPacket = &StatusPacket{[]PeerStatus{PeerStatus{rumor.Origin, rumor.ID+1}}}
			gossiper.SafeRumors.mux.Unlock()
			return "present"
		}  else {
			return "future"
		}
	} 

	knownOrigin := false

	// Check our statusPacket to see if we already have the rumor
	for index,peerStatus := range gossiper.StatusPacket.Want {
		// If we have already seen this identifier 
		if(rumor.Origin == peerStatus.Identifier) {
			knownOrigin = true
		}
		// This is the next message id (not some future message)
		if(knownOrigin) {
			if(rumor.ID == peerStatus.NextID) {
				gossiper.StatusPacket.Want[index].NextID += 1

				gossiper.SafeRumors.mux.Lock()
				// store rumor in rumor array
				gossiper.SafeRumors.RumorMessages[rumor.Origin] = append(gossiper.SafeRumors.RumorMessages[rumor.Origin], rumor)
				if(!isRouteMessage && rumor.Origin != gossiper.Name) {
					gossiper.RumorMessages = append(gossiper.RumorMessages, rumor)
				}
				gossiper.SafeRumors.mux.Unlock()
				return "present"
			} else if(rumor.ID > peerStatus.NextID) {
				return "future"
			} else if(rumor.ID < peerStatus.NextID) {
				return "past"
			}
		}
	}

	if(!knownOrigin) {
		if(rumor.ID == 1) {
			gossiper.StatusPacket.Want = append(gossiper.StatusPacket.Want, PeerStatus{rumor.Origin, rumor.ID+1})
			gossiper.SafeRumors.mux.Lock()
			gossiper.SafeRumors.RumorMessages[rumor.Origin] = append(gossiper.SafeRumors.RumorMessages[rumor.Origin], rumor)
			gossiper.SafeRumors.mux.Unlock()
			if(!isRouteMessage && rumor.Origin != gossiper.Name) {
				gossiper.RumorMessages = append(gossiper.RumorMessages, rumor)
			}
			return "present"
		} else {
			return "future"
		}
	}

	return ""
}   

func getRumorFromArray(rumorArray []RumorMessage, id uint32) RumorMessage { 
	var nilRumor RumorMessage

	for _,rumor := range rumorArray {
		if(rumor.ID == id) {
			return rumor
		}
	}

	return nilRumor
}

/*
	Compares gossiper's status (Want) with that of the sender of the peerStatus
	If there are some messages that we received and the other gossiper has not seen, we return (next rumor he wants, nil)
	If there are some that the other peer has and we don't, we return (nil, status)
	If we are in sync, we return nil, nil
*/
func compareStatus(otherPeerStatus []PeerStatus, peerAddress string) (*RumorMessage, *StatusPacket) {

	var nilRumor *RumorMessage
	var nilStatus *StatusPacket
	var foundIdentifier bool

	// Check if our statusPacket is empty
	if(len(gossiper.StatusPacket.Want) == 0) {
		return nilRumor, nilStatus
	}

	var rumorToSend RumorMessage

	gossiper.SafeRumors.mux.Lock()

	// Check if other Peer status packet is empty (send him the any rumor with ID = 1 if it is the case)
	if(len(otherPeerStatus) == 0) {
		for _, key := range reflect.ValueOf(gossiper.SafeRumors.RumorMessages).MapKeys() {
			// Send first rumor of any key
			rumorToSend = gossiper.SafeRumors.RumorMessages[key.Interface().(string)][0]
			gossiper.SafeRumors.mux.Unlock()
			return &rumorToSend, nilStatus
		} 
	}

	sendMsgStatus := false

	// Check if he needs no rumors :
	for _, myPS := range gossiper.StatusPacket.Want {
		// Check if other peer has this identifier
		foundIdentifier = false
		for _, otherPS := range otherPeerStatus {
			if(otherPS.Identifier == myPS.Identifier) {
				foundIdentifier = true
				// It has this identifier, so check if the nextID is lower than ours
				if(otherPS.NextID < myPS.NextID) {
					// Send the rumor with the other peer nextID
					rumorToSend = getRumorFromArray(gossiper.SafeRumors.RumorMessages[myPS.Identifier], otherPS.NextID)
					// Just to test :
					if(rumorToSend.ID != otherPS.NextID) {
						fmt.Println("ERROR : rumor is not in order of ID's !")
					}

					gossiper.SafeRumors.mux.Unlock()
					return &rumorToSend, nilStatus
				} else if(otherPS.NextID > myPS.NextID) {
					sendMsgStatus = true
				}
			}
		}
		// Peer does not have any rumors of this identifier
		if(!foundIdentifier) {
			// Return first rumor of this identifier :
			rumorToSend = getRumorFromArray(gossiper.SafeRumors.RumorMessages[myPS.Identifier], 1)
			// Just to test :
			if(rumorToSend.ID != 1) {
				fmt.Println("ERROR : first rumor is not ID 1 !")
			}

			gossiper.SafeRumors.mux.Unlock()
			return &rumorToSend, nilStatus
		}
	}

	// other gossiper has messages that we don't have and we don't have anything more to send
	if(sendMsgStatus) {
		//unlock map
		gossiper.SafeRumors.mux.Unlock()
		return nilRumor, gossiper.StatusPacket
	}

	// Check if we need rumors from other peer
	for _, otherPS := range otherPeerStatus {
		foundIdentifier = false
		for _, myPS := range gossiper.StatusPacket.Want {
			if(otherPS.Identifier == myPS.Identifier) {
				foundIdentifier = true
				if(myPS.NextID < otherPS.NextID) { // other gossiper has messages that we don't have
					sendMsgStatus = true
				}
			}
		}

		if(!foundIdentifier) { // Other Peer has received a message from an identifier we didn't have
			sendMsgStatus = true
		}
	}

	// other gossiper has messages that we don't have and we don't have anything more to send
	if(sendMsgStatus) {
		//unlock map
		gossiper.SafeRumors.mux.Unlock()
		return nilRumor, gossiper.StatusPacket
	}

	// We have the same rumors as peer (status packets are the same)
	//unlock map
	gossiper.SafeRumors.mux.Unlock()
	return nilRumor, nilStatus
}


// This function makes a new timer and sets the timeout to ONE second. If the timer finishes it calls the method to remove the timer.
func makeTimer(peerAddress string, rumorMessage RumorMessage) {

	respTimer := ResponseTimer{
		Timer: time.NewTimer(time.Second), 
		Responder: peerAddress,
	}

	gossiper.SafeTimers.mux.Lock()
	_, peerTimerExists := gossiper.SafeTimers.ResponseTimers[peerAddress]

	if(!peerTimerExists) {
		gossiper.SafeTimers.ResponseTimers[peerAddress] = []ResponseTimer{}
	} 

	gossiper.SafeTimers.ResponseTimers[peerAddress] = append(gossiper.SafeTimers.ResponseTimers[peerAddress], respTimer)
	gossiper.SafeTimers.mux.Unlock()

	go func() {
		<- respTimer.Timer.C

		removeFinishedTimer(peerAddress)

		/*if(rand.Int() % 2 == 0) {
	        go rumormongering(rumorMessage, true)
	    }*/
	        		
	}()
}

// This function makes a new timer and sets the timeout to 5 seconds. If the timer finishes it calls the method to remove the timer.
func makeDataRequestTimer(fileOrigin string, dataRequest DataRequest) {

	dataRespTimer := ResponseTimer{
		Timer: time.NewTimer(5*time.Second), 
		Responder: fileOrigin,
	}

	gossiper.SafeDataRequestTimers.mux.Lock()
	_, peerTimerExists := gossiper.SafeDataRequestTimers.ResponseTimers[fileOrigin]

	if(!peerTimerExists) {
		gossiper.SafeDataRequestTimers.ResponseTimers[fileOrigin] = []ResponseTimer{}
	} 

	gossiper.SafeDataRequestTimers.ResponseTimers[fileOrigin] = append(gossiper.SafeDataRequestTimers.ResponseTimers[fileOrigin], dataRespTimer)
	gossiper.SafeDataRequestTimers.mux.Unlock()

	go func() {
		<- dataRespTimer.Timer.C

		removeFinishedDataRequestTimer(fileOrigin)

		// Send the request again
		fmt.Println("TIMEOUT!")
	    sendDataRequestToSpecificPeer(dataRequest, getAddressFromRoutingTable(fileOrigin))//gossiper.RoutingTable[fileOrigin])
	    makeDataRequestTimer(fileOrigin, dataRequest)	
	}()
}

// This method removes the oldest timer from the peer passed as argument
func removeFinishedTimer(peerAddress string) {

	gossiper.SafeTimers.mux.Lock()

	if(len(gossiper.SafeTimers.ResponseTimers[peerAddress]) == 0) {
		gossiper.SafeTimers.mux.Unlock()
		return
	} else if(len(gossiper.SafeTimers.ResponseTimers[peerAddress]) == 1) {
		delete(gossiper.SafeTimers.ResponseTimers, peerAddress)
	} else {
		gossiper.SafeTimers.ResponseTimers[peerAddress] = gossiper.SafeTimers.ResponseTimers[peerAddress][1:]
	}

	gossiper.SafeTimers.mux.Unlock()
}

func removeFinishedDataRequestTimer(peerAddress string) {
	gossiper.SafeDataRequestTimers.mux.Lock()

	if(len(gossiper.SafeDataRequestTimers.ResponseTimers[peerAddress]) == 0) {
		gossiper.SafeDataRequestTimers.mux.Unlock()
		return
	} else if(len(gossiper.SafeDataRequestTimers.ResponseTimers[peerAddress]) == 1) {
		delete(gossiper.SafeDataRequestTimers.ResponseTimers, peerAddress)
	} else {
		gossiper.SafeDataRequestTimers.ResponseTimers[peerAddress] = gossiper.SafeDataRequestTimers.ResponseTimers[peerAddress][1:]
	}

	gossiper.SafeDataRequestTimers.mux.Unlock()
}

// This method returns true if the peer passed as argument is sending a status response, otherwise it returns false
func isStatusResponse(peerAddress string) bool {

	gossiper.SafeTimers.mux.Lock()
	_, hasTimer := gossiper.SafeTimers.ResponseTimers[peerAddress]
	gossiper.SafeTimers.mux.Unlock()

	if(!hasTimer) {
		return false
	} else {
		gossiper.SafeTimers.mux.Lock()
		gossiper.SafeTimers.ResponseTimers[peerAddress][0].Timer.Stop()
		gossiper.SafeTimers.mux.Unlock()
		return true
	}	
}

func isDataResponse(fileOrigin string) bool {

	gossiper.SafeDataRequestTimers.mux.Lock()
	_, hasTimer := gossiper.SafeDataRequestTimers.ResponseTimers[fileOrigin]
	gossiper.SafeDataRequestTimers.mux.Unlock()

	if(!hasTimer) {
		return false
	} else {
		gossiper.SafeDataRequestTimers.mux.Lock()
		gossiper.SafeDataRequestTimers.ResponseTimers[fileOrigin][0].Timer.Stop()
		gossiper.SafeDataRequestTimers.mux.Unlock()
		return true
	}
}

// Function to update the routing table of our gossiper
func updateRoutingTable(rumor RumorMessage, address string) bool {

	tableUpdated := false

	if(rumor.Origin == gossiper.Name) {
		return tableUpdated
	}

	gossiper.SafeRoutingTables.mux.Lock()

	if(len(gossiper.SafeRoutingTables.RoutingTable[rumor.Origin]) == 0) { // We do not know a path to this origin
		gossiper.SafeRoutingTables.RoutingTable[rumor.Origin] = address
		tableUpdated = true
	} else if(gossiper.SafeRoutingTables.RoutingTable[rumor.Origin] != address) { // The address needs to be updated
		gossiper.SafeRoutingTables.RoutingTable[rumor.Origin] = address
		tableUpdated = true
	} else {
		tableUpdated = false
	}

	gossiper.SafeRoutingTables.mux.Unlock()
	return tableUpdated
}    	

// Create a new gossiper
func NewGossiper(UIPort, gossipPort, name string, peers string) *Gossiper {
	
	// Listen on UIPort
	udpUIPort, err := net.ResolveUDPAddr("udp4", UIPort)
	isError(err)
	udpUIPortConn, err := net.ListenUDP("udp4", udpUIPort)
	isError(err)

	
	// Listen on GossipPort
	udpGossipPort, err := net.ResolveUDPAddr("udp4", gossipPort)
	isError(err)
	udpGossipConn, err := net.ListenUDP("udp4", udpGossipPort)
	isError(err)

	var p []string

	if(len(peers) != 0) {
		p = strings.Split(peers, ",")
	}

	return &Gossiper{
		UIPortAddr: udpUIPort,
		UIPortConn: udpUIPortConn,
		GossipPortAddr: udpGossipPort,
		GossipPortConn: udpGossipConn,
		Name: name,
		Peers_as_single_string: peers,
		Peers: p,
		StatusPacket: &StatusPacket{},
		SafeRumors: SafeRumor{
			RumorMessages: make(map[string][]RumorMessage),
		},
		RumorMessages: []RumorMessage{},
		PrivateMessages: []PrivateMessage{},
		SafeTimers: SafeTimer{
			ResponseTimers: make(map[string][]ResponseTimer),
		},
		LastRumorSentIndex: -1,
		LastPrivateSentIndex: -1,
		LastNodeSentIndex: -1,
		SentCloseNodes: []string{},
		SafeNextClientMessageIDs: SafeNextClientMessageID{
			NextClientMessageID: 1,
		},
		SafeRoutingTables: SafeRoutingTable{
			RoutingTable: make(map[string]string),
		},
		SafeIndexedFiles: SafeIndexedFile{
			IndexedFiles: make(map[string]File),
		},
		SafeDataRequestTimers: SafeTimer{
			ResponseTimers: make(map[string][]ResponseTimer),
		},
		// map of origin of requests to files they are requesting
		SafeRequestOriginToFileAndIndexes: SafeRequestOriginToFileAndIndex{
			RequestOriginToFileAndIndex: make(map[string][]FileAndIndex),
		},
		// map of origin of files to files I am requesting
		SafeRequestDestinationToFileAndIndexes: SafeRequestDestinationToFileAndIndex{
			RequestDestinationToFileAndIndex: make(map[string][]FileAndIndex),
		},
		SafeMySearchRequests: SafeMySearchRequest{
			SearchRequests: make(map[string]int),
		},
	}
}

func printStatusReceived(peerStatus []PeerStatus, peerAddress string) {

	//fmt.Print("STATUS from ", peerAddress, " ") 

	if(len(peerStatus) == 0) {
		return
	}

	// JUST FOR TEST PURPOSES TESTING
	/*for _,ps := range peerStatus {
		fmt.Print("peer ", ps.Identifier, " nextID ", ps.NextID, " ")
	}
	fmt.Println()*/
}

func getAddressFromRoutingTable(dest string) string {

	gossiper.SafeRoutingTables.mux.Lock()

	if(len(gossiper.SafeRoutingTables.RoutingTable[dest]) == 0) {
		fmt.Println("Error : No such destination :", dest)
		gossiper.SafeRoutingTables.mux.Unlock()
		return ""
	} else {
		gossiper.SafeRoutingTables.mux.Unlock()
		return gossiper.SafeRoutingTables.RoutingTable[dest]
	}
}
