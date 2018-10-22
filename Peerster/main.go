package main

import(
	"fmt"
	"flag"
	"net"
	"protobuf"
	"strings"
	"math/rand"
	"time"
	"sync"
	//"mux"
    //"handlers"
    "simplejson"
    "net/http"
    "regexp"
    "reflect"
)

type SimpleMessage struct {
	OriginalName string
	RelayPeerAddr string
	Contents string
}

type RumorMessage struct {
	Origin string
	ID uint32
	Text string
}

type PrivateMessage struct {
    Origin string
    ID uint32
    Text string
    Destination string
    HopLimit uint32
}

type PeerStatus struct {
	Identifier string
	NextID uint32
} 

type StatusPacket struct {
	Want []PeerStatus
}

type GossipPacket struct {
	Simple *SimpleMessage
	Rumor *RumorMessage
	Status *StatusPacket
	Private *PrivateMessage
}

type Gossiper struct {
	UIPortAddr *net.UDPAddr
	UIPortConn *net.UDPConn
	GossipPortAddr *net.UDPAddr
	GossipPortConn *net.UDPConn
	Name string
	Peers_as_single_string string
	Peers []string
	StatusPacket *StatusPacket
	SafeRumors SafeRumor
	RumorMessages []RumorMessage
	LastRumor RumorMessage
	SafeTimers SafeTimer
	TimersBeingChanged bool
	LastRumorSentIndex int
	StatusOfGUI map[string]uint32
	LastNodeSentIndex int 
	SentCloseNodes []string
	NextClientMessageID uint32
	RoutingTable map[string]string
}

type SafeRumor struct {
	RumorMessages map[string][]RumorMessage 
	mux sync.Mutex
}

type SafeTimer struct {
	ResponseTimers map[string][]ResponseTimer
	mux sync.Mutex
}

type ResponseTimer struct {
	Timer *time.Timer
	Responder string 
}

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

// Function to update the list of known peers of our gossiper
func updatePeerList(gossiper *Gossiper, peerAddr string) {
	if(!contains(gossiper.Peers, peerAddr)) {
		gossiper.Peers = append(gossiper.Peers, peerAddr)
		if(len(gossiper.Peers_as_single_string) != 0) {
			gossiper.Peers_as_single_string = gossiper.Peers_as_single_string + ","
	    }
	    gossiper.Peers_as_single_string = gossiper.Peers_as_single_string + peerAddr
    }
}

// Function to update the routing table of our gossiper
func updateRoutingTable(gossiper *Gossiper, rumor RumorMessage, address string) bool {
	if(rumor.Origin == gossiper.Name) {
		return false
	}

	if(len(gossiper.RoutingTable[rumor.Origin]) == 0) { // We do not know a path to this origin
		gossiper.RoutingTable[rumor.Origin] = address
		return true
	} else if(gossiper.RoutingTable[rumor.Origin] != address) { // The address needs to be updated
		gossiper.RoutingTable[rumor.Origin] = address
		return true
	} else {
		return false
	}
}

/* 
	This method updates our rumor array by appending the rumor if it is unseen.
	This method also updates in our status packet the "next id" of the peer that we received this rumor from.
	Returns a string : "past", "present", "future"
*/
func updateStatusAndRumorArray(gossiper *Gossiper, rumor RumorMessage, isRouteMessage bool) string {
	//fmt.Println("inside update status and rumor array with :", rumor)
	// This is the first rumor we receive
	if(gossiper.StatusPacket == nil) {
		//fmt.Println("Table is nil !")
		if(rumor.ID == 1){
			//fmt.Println("First rumor", rumor)
			// add rumor to rumors from this origin
			gossiper.SafeRumors.RumorMessages[rumor.Origin] = []RumorMessage{rumor}
			if(!isRouteMessage) {
				gossiper.RumorMessages = []RumorMessage{rumor}
			}
			gossiper.StatusPacket = &StatusPacket{[]PeerStatus{PeerStatus{rumor.Origin, rumor.ID+1}}}
			return "present"
			// return true
		}  else {
			//fmt.Println("rumor is future...", rumor)
			return "future"
			//return false
		}
	} 

	knownOrigin := false
	isFutureID := false
	isPastID := false

	// Check our statusPacket to see if we already have the rumor
	for index,peerStatus := range gossiper.StatusPacket.Want {
		// If we have already seen this identifier 
		if(rumor.Origin == peerStatus.Identifier) {
			knownOrigin = true
		}
		// This is the next message id (not some future message)
		if(rumor.Origin == peerStatus.Identifier) {
			if(rumor.ID == peerStatus.NextID) {
				gossiper.StatusPacket.Want[index].NextID += 1
				// store rumor in rumor array
				gossiper.SafeRumors.RumorMessages[rumor.Origin] = append(gossiper.SafeRumors.RumorMessages[rumor.Origin], rumor)
				if(!isRouteMessage) {
					gossiper.RumorMessages = append(gossiper.RumorMessages, rumor)
				}
				return "present"
				//return true
			} else if(rumor.ID > peerStatus.NextID) {
				//fmt.Println("rumor is future !!", rumor)
				isFutureID = true
			} else if(rumor.ID < peerStatus.NextID) {
				isPastID = true
			}
		}
	}

	if(!knownOrigin) {
		//fmt.Println("Unknown origin")
		if(rumor.ID == 1) {
			gossiper.StatusPacket.Want = append(gossiper.StatusPacket.Want, PeerStatus{rumor.Origin, rumor.ID+1})
			gossiper.SafeRumors.RumorMessages[rumor.Origin] = append(gossiper.SafeRumors.RumorMessages[rumor.Origin], rumor)
			if(!isRouteMessage) {
				gossiper.RumorMessages = append(gossiper.RumorMessages, rumor)
			}
			return "present"
			//return true
		} else {
			//fmt.Println("x. rumor is future:", rumor.ID, "", rumor.Origin, "", rumor)
			return "future"
			//return false
		}
	}
    // The rumor is either already seen or this is not the next rumor we want from this origin
    if(isFutureID && !isPastID) {
    	return "future"
    } else if(!isFutureID && isPastID) {
    	return  "past"
    } else {
    	fmt.Println("Error : Future and past id", rumor.ID, "with", gossiper.StatusPacket.Want)
    	return ""
    }
    //return false
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
func compareStatus(gossiper *Gossiper, otherPeerStatus []PeerStatus, peerAddress string) (*RumorMessage, *StatusPacket) {

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
		SafeTimers: SafeTimer{
			ResponseTimers: make(map[string][]ResponseTimer),
		},
		LastRumorSentIndex: -1,
		StatusOfGUI: make(map[string]uint32),
		LastNodeSentIndex: -1,
		SentCloseNodes: []string{},
		RoutingTable: make(map[string]string),
	}
}

/*
	 ----------------------------BEGIN SIMPLE BROADCAST-----------------------------------------
*/

func simpleListenUIPort(gossiper *Gossiper) {

	defer gossiper.UIPortConn.Close()
 
    packetBytes := make([]byte, 1024)
 
    for true {

        _,_,err := gossiper.UIPortConn.ReadFromUDP(packetBytes)
        isError(err)

        receivedPkt := &GossipPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

        if(receivedPkt.Simple != nil) {

			msg :=  receivedPkt.Simple.Contents

	        fmt.Println("CLIENT MESSAGE", msg) 
	        fmt.Println("PEERS :", gossiper.Peers_as_single_string)

			simpleMessage := SimpleMessage{
	        	OriginalName: gossiper.Name,
				RelayPeerAddr: gossiper.GossipPortAddr.String(),
				Contents: msg,
        	}

        	sendSimpleMsgToAllPeersExceptSender(gossiper, simpleMessage, "")
		}
    }
}

func simpleListenGossipPort(gossiper *Gossiper) {
	defer gossiper.GossipPortConn.Close()
	packetBytes := make([]byte, 1024)


	for true {

        _,addr,err := gossiper.GossipPortConn.ReadFromUDP(packetBytes)
        isError(err)

		receivedPkt := &GossipPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

		peerAddr := addr.String()	
		updatePeerList(gossiper, peerAddr)	

        if(receivedPkt.Simple != nil) {

			msg :=  receivedPkt.Simple.Contents
			name := receivedPkt.Simple.OriginalName

	        fmt.Println("SIMPLE MESSAGE origin", name, "from", peerAddr, "contents", msg) 
	        fmt.Println("PEERS :", gossiper.Peers_as_single_string)

			simpleMessage := SimpleMessage{
	        	OriginalName: name,
				RelayPeerAddr: gossiper.GossipPortAddr.String(),
				Contents: msg,
        	}

        	sendSimpleMsgToAllPeersExceptSender(gossiper, simpleMessage, peerAddr)
		}
    }
}

func sendSimpleMsgToAllPeersExceptSender(gossiper *Gossiper, simpleMessage SimpleMessage, sender string) {

	if(len(gossiper.Peers) == 0) {
		return 
	}

	for _, address := range gossiper.Peers {
		if(address != sender) {
	   		// Encode message
			packet := &GossipPacket{Simple: &simpleMessage}
			packetBytes, err := protobuf.Encode(packet)
			isError(err)
		 	
		 	// Start UDP connection
		    peerAddr, err := net.ResolveUDPAddr("udp4", address)
		    isError(err)
		 	
	        _,err = gossiper.GossipPortConn.WriteTo(packetBytes, peerAddr)
	        isError(err)
	 	}
	}

	return 
}

/*
	 ----------------------------END SIMPLE BROADCAST----------------------------------------------------
*/

func sendPacketToSpecificPeer(gossiper *Gossiper, packet GossipPacket, address string) {
	packetBytes, err := protobuf.Encode(&packet)
	isError(err)
		 	
	// Start UDP connection
	peerAddr, err := net.ResolveUDPAddr("udp4", address)
	isError(err)
		 	
	_,err = gossiper.GossipPortConn.WriteTo(packetBytes, peerAddr)
	isError(err)
}

func sendRumorMsgToSpecificPeer(gossiper *Gossiper, rumorMessage RumorMessage, address string) {
	// Encode message
	packet := GossipPacket{Rumor: &rumorMessage}
	/*packetBytes, err := protobuf.Encode(packet)
	isError(err)
		 	
	// Start UDP connection
	peerAddr, err := net.ResolveUDPAddr("udp4", address)
	isError(err)

	fmt.Println("MONGERING with", address) 
		 	
	_,err = gossiper.GossipPortConn.WriteTo(packetBytes, peerAddr)
	isError(err)*/
	sendPacketToSpecificPeer(gossiper, packet, address)
}

func sendPrivateMsgToSpecificPeer(gossiper *Gossiper, privateMessage PrivateMessage, address string) {
	// Encode message
	packet := GossipPacket{Private: &privateMessage}
	sendPacketToSpecificPeer(gossiper, packet, address)
}

func sendStatusMsgToSpecificPeer(gossiper *Gossiper, address string) {
	// Encode message
	packet := GossipPacket{Status: gossiper.StatusPacket}
	sendPacketToSpecificPeer(gossiper, packet, address)
}

// This function makes a new timer and sets the timeout to ONE second. If the timer finishes it calls the method to remove the timer.
func makeTimer(gossiper *Gossiper, peerAddress string, rumorMessage RumorMessage) {

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

		removeFinishedTimer(gossiper, peerAddress)

		if(rand.Int() % 2 == 0) {
	        go rumormongering(gossiper, rumorMessage, true)
	    }
	        		
	}()
}

// This method removes the oldest timer from the peer passed as argument
func removeFinishedTimer(gossiper *Gossiper, peerAddress string) {

	gossiper.SafeTimers.mux.Lock()

	if(len(gossiper.SafeTimers.ResponseTimers[peerAddress]) == 0) {
		return
	} else if(len(gossiper.SafeTimers.ResponseTimers[peerAddress]) == 1) {
		delete(gossiper.SafeTimers.ResponseTimers, peerAddress)
	} else {
		gossiper.SafeTimers.ResponseTimers[peerAddress] = gossiper.SafeTimers.ResponseTimers[peerAddress][1:]
	}

	gossiper.SafeTimers.mux.Unlock()
}

// This method returns true if the peer passed as argument is sending a status response, otherwise it returns false
func isStatusResponse(gossiper *Gossiper, peerAddress string) bool {

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

func generatePeriodicalRouteMessage(gossiper *Gossiper, rtimer int) {
	var ticker *time.Ticker
	var luckyPeer string
	var routeMessage RumorMessage
	ticker = time.NewTicker(time.Second)

	for {
		for range ticker.C {
			if(len(gossiper.Peers) > 0) {
				routeMessage = RumorMessage{
					Origin: gossiper.Name,
					ID: gossiper.NextClientMessageID, // WHAT SHOULD THE ID OF ROUTE MESSAGES BE ?
					Text: "",
				}

				gossiper.NextClientMessageID++

				rand.Seed(time.Now().UTC().UnixNano())
	    		luckyPeer = gossiper.Peers[rand.Intn(len(gossiper.Peers))]
	    		sendRumorMsgToSpecificPeer(gossiper, routeMessage, luckyPeer)
	    		makeTimer(gossiper, luckyPeer, routeMessage)
	    	}
		}
	}
}

// Anti entropy process
func antiEntropy(gossiper *Gossiper) {
	var ticker *time.Ticker
	var luckyPeer string
	ticker = time.NewTicker(time.Second)

	for {
		for range ticker.C {
			if(len(gossiper.Peers) > 0) {
				rand.Seed(time.Now().UTC().UnixNano())
	    		luckyPeer = gossiper.Peers[rand.Intn(len(gossiper.Peers))]
	    		sendStatusMsgToSpecificPeer(gossiper, luckyPeer)
	    	}
		}
	}
}

// RumorMongering process
func rumormongering(gossiper *Gossiper, rumorMessage RumorMessage, isRandom bool) {

	rand.Seed(time.Now().UTC().UnixNano())

	if(len(gossiper.Peers) == 0) {
		return
	}

    luckyPeer := gossiper.Peers[rand.Intn(len(gossiper.Peers))]

    if(isRandom) {
    	//fmt.Println("FLIPPED COIN sending rumor to", luckyPeer)
    }

	sendRumorMsgToSpecificPeer(gossiper, rumorMessage, luckyPeer)
	makeTimer(gossiper, luckyPeer, rumorMessage)
}

func printStatusReceived(gossiper *Gossiper, peerStatus []PeerStatus, peerAddress string) {

	fmt.Print("STATUS from ", peerAddress, " ") 

	if(len(peerStatus) == 0) {
		return
	}

	for _,ps := range peerStatus {
		fmt.Print("peer ", ps.Identifier, " nextID ", ps.NextID, " ")
	}
	fmt.Println()
}

func getAddressFromRoutingTable(gossiper *Gossiper, dest string) string {
	if(len(gossiper.RoutingTable[dest]) == 0) {
		fmt.Println("Error : No such destination :", dest)
	}

	return gossiper.RoutingTable[dest]
}

func listenUIPort(gossiper *Gossiper) {

	defer gossiper.UIPortConn.Close()
 	
    packetBytes := make([]byte, 1024)

	gossiper.NextClientMessageID = 1
 
    for true {

        _,_,err := gossiper.UIPortConn.ReadFromUDP(packetBytes)
        isError(err)

        receivedPkt := &GossipPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

        if(receivedPkt.Rumor != nil) {

	        msg := receivedPkt.Rumor.Text

	        fmt.Println("CLIENT MESSAGE", msg) 
	        fmt.Println("PEERS", gossiper.Peers_as_single_string)

        	rumorMessage := RumorMessage{
        		Origin: gossiper.Name, 
        		ID: gossiper.NextClientMessageID,
        		Text: msg,
        	}

        	gossiper.LastRumor = rumorMessage
        	gossiper.NextClientMessageID ++
        	stateID := updateStatusAndRumorArray(gossiper, rumorMessage, false)

        	if(len(gossiper.Peers) > 0) {
        		if(stateID == "present") {
        			go rumormongering(gossiper, rumorMessage, false)
        		} else {
	        		if(rand.Int() % 2 == 0) {
	        			go rumormongering(gossiper, rumorMessage, true)
	        		}
	        	}
        	}
		} else if(receivedPkt.Private != nil) {
			dest := receivedPkt.Private.Destination

			privateMsg := PrivateMessage{
                Origin : gossiper.Name,
                ID : 0,
                Text : receivedPkt.Private.Text,
                Destination : dest,
                HopLimit : 10,
            }

			address := getAddressFromRoutingTable(gossiper, dest)
			sendPrivateMsgToSpecificPeer(gossiper, privateMsg, address)
		} 
    }
}

func listenGossipPort(gossiper *Gossiper) {

	defer gossiper.GossipPortConn.Close()
	packetBytes := make([]byte, 1024)
	var tableUpdated bool
	var stateID string

	for true {	

        _,addr,err := gossiper.GossipPortConn.ReadFromUDP(packetBytes)
        isError(err)

		receivedPkt := &GossipPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

		peerAddr := addr.String()		
		updatePeerList(gossiper, peerAddr)
		fmt.Println("PEERS", gossiper.Peers_as_single_string)

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
				stateID = updateStatusAndRumorArray(gossiper, rumorMessage, false)
        		fmt.Println("RUMOR origin", origin, "from", peerAddr, "ID", id, "contents", msg) 
        	} else {
        		stateID = updateStatusAndRumorArray(gossiper, rumorMessage, true)
        	}

        	tableUpdated = false

        	if(stateID == "present" || stateID == "future") {
        		fmt.Println("Rumor is :", stateID)
	        	tableUpdated = updateRoutingTable(gossiper, rumorMessage, peerAddr)
	        }

        	if(tableUpdated) {
        		fmt.Println("DSDV", origin, peerAddr)
        	}

        	sendStatusMsgToSpecificPeer(gossiper, peerAddr)
        	
        	if(len(gossiper.Peers) > 0) {
        		if(stateID == "present") {
        			go rumormongering(gossiper, rumorMessage, false)
        		} else {
	        		if(rand.Int() % 2 == 0) {
	        			go rumormongering(gossiper, rumorMessage, true)
	        		}
	        	}
        	}
	    } else if(receivedPkt.Status != nil) {

	    	isResponse := isStatusResponse(gossiper, peerAddr)
	    	
	    	if(isResponse) {
	    		removeFinishedTimer(gossiper, peerAddr)
	    	}

			peerStatus :=  receivedPkt.Status.Want

			printStatusReceived(gossiper, peerStatus, peerAddr)
			rumorToSend, statusToSend := compareStatus(gossiper, peerStatus, peerAddr)

			if(rumorToSend != nil) {

				sendRumorMsgToSpecificPeer(gossiper, *rumorToSend, peerAddr)
				makeTimer(gossiper, peerAddr, *rumorToSend)

			} else if(statusToSend != nil) {

				sendStatusMsgToSpecificPeer(gossiper, peerAddr)

			} else {
				fmt.Println("IN SYNC WITH", peerAddr)

				if(rand.Int() % 2 == 0) {
					// Reading rumor message map ?
					if(len(gossiper.SafeRumors.RumorMessages) > 0) {
		        		go rumormongering(gossiper, gossiper.LastRumor, true)
		        	}
	        	}
			}
		} else if(receivedPkt.Private != nil) {
			origin := receivedPkt.Private.Origin
			dest := receivedPkt.Private.Destination
			hopLimit := receivedPkt.Private.HopLimit
			msg := receivedPkt.Private.Text

			if(dest == gossiper.Name) {
				fmt.Println("PRIVATE origin", origin, "hop-limit", hopLimit, "contents", msg)
			} else {

				//fmt.Println("Rerouting", msg, "from", origin, "to", dest)

				privateMsg := PrivateMessage{
	                Origin : origin,
	                ID : receivedPkt.Private.ID,
	                Text : msg,
	                Destination : dest,
	                HopLimit : hopLimit,
            	}

            	address := getAddressFromRoutingTable(gossiper, dest)

            	if(hopLimit != 0) {
					privateMsg.HopLimit--
					sendPrivateMsgToSpecificPeer(gossiper, privateMsg, address)
				}
			}
		}
    }
}

// Send ID to frontend
func IDHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
    json := simplejson.New()
	json.Set("ID", gossiper.Name)

	payload, err := json.MarshalJSON()
	isError(err)

	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
   
}


/* This method does the following :
	1. Check if frontend wants to update its messages :
	2. If yes :
		3. Check the last rumor we sent : "lastRumorSentIndex"
		4. If len(gossiper.RumorMessages)-1 > lastRumorSentIndex
			5. Send all rumors from lastRumorSentIndex to len(gossiper.RumorMessages)
		6. Else do nothing
	7. If no (frontend sends a new message) :
		8. Do as in "listenUIPort"
*/

func MessageHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

    if(r.FormValue("Update") == "") { // Send messages to frontend
    	// Send last messages, keep track of index of last one sent
	    json := simplejson.New()

	    nb_messages := len(gossiper.RumorMessages)

	    if(nb_messages-1 <= gossiper.LastRumorSentIndex) {
	    	return
	    }

	    messageArray := []string{}
	    var msg string

	    for i := gossiper.LastRumorSentIndex + 1; i < nb_messages; i++ {
	    	msg = gossiper.RumorMessages[i].Origin + " : " + gossiper.RumorMessages[i].Text
	    	messageArray = append(messageArray, msg)
	    }

	    gossiper.LastRumorSentIndex = nb_messages - 1

		json.Set("Message", messageArray)

		payload, err := json.MarshalJSON()
		isError(err)

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	} else if(r.FormValue("Message") != ""){ // Get message from frontend
		//  Do as in "listenUIPort" 
		msg := r.FormValue("Message")

		fmt.Println("CLIENT MESSAGE", msg) 	
		fmt.Println("PEERS", gossiper.Peers_as_single_string)

		rumorMessage := RumorMessage{
			Origin: gossiper.Name, 
	        ID: gossiper.NextClientMessageID,
	        Text: msg,
		}

	    gossiper.NextClientMessageID ++

		stateID := updateStatusAndRumorArray(gossiper, rumorMessage, false)
		
		if(len(gossiper.Peers) > 0) {
			if(stateID == "present") {
				go rumormongering(gossiper, rumorMessage, false)
			} else {
				if(rand.Int() % 2 == 0) {
					go rumormongering(gossiper, rumorMessage, true)
		        }
			}
		}
	} else if(r.FormValue("PrivateMessage") != "") {
		fmt.Println("Received private message :", r.FormValue("PrivateMessage"), "to", r.FormValue("Destination"))
		dest := r.FormValue("Destination")

		privateMsg := PrivateMessage{
			Origin : gossiper.Name,
			ID : 0,
			Text : r.FormValue("PrivateMessage"),
			Destination : dest,
			HopLimit : 10,
        }

		address := getAddressFromRoutingTable(gossiper, dest)
		sendPrivateMsgToSpecificPeer(gossiper, privateMsg, address)
	}
    
}


/* This method does the follwing
	1. Check if frontend wants to update its nodes :
	2. If yes :
		3. Check the last node we sent : "LastNodeSentIndex"
		4. If len(gossiper.Peers)-1 > LastNodeSentIndex
			5. Send all nodes from LastNodeSentIndex to len(gossiper.Peers)
		6. Else do nothing
	7. If no (frontend sends a new node) :
		8. Check syntax of ip:port
		9. If syntax is ok, add to Peers and Peers_as_single_string
*/

func NodeHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

	if(r.FormValue("Update") == "") { // Send nodes to frontend
        json := simplejson.New()
	    nodeArray := []string{}
	    nb_peers := len(gossiper.Peers)

	    if(nb_peers-1 <= gossiper.LastNodeSentIndex) {
	    	return
	    }

	    nodeArray = []string{}

	    for i := gossiper.LastNodeSentIndex + 1; i < nb_peers; i++ {
	    	nodeArray = append(nodeArray, gossiper.Peers[i])
	    }

	    gossiper.LastNodeSentIndex = nb_peers - 1

	    json.Set("Node", nodeArray)

		payload, err := json.MarshalJSON()
		isError(err)

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)

    } else {
        newNodeAddr := r.FormValue("Node")
        // Check syntax of ip:port
        // Should match xxx.xxx.xxx.xxx:xxxxxxxxx...
        r := "^((25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[0-9]{2}|[0-9]).)((25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[0-9]{2}|[0-9]).)((25[0-5]|2[0-4][0-9]|"
		r = r + "[0-1][0-9]{2}|[0-9]{2}|[0-9]).)((25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[0-9]{2}|[0-9]):)([0-9]+)$"
		match, _ := regexp.MatchString(r, newNodeAddr)

		if(match) {
			updatePeerList(gossiper, newNodeAddr)
		}
    }   
}

func CloseNodeHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

	if(r.FormValue("Update") == "") { // client wants to get new close nodes
        json := simplejson.New()


	    // LOCK ROUTING TABLE MAP ?


	    close_nodes := reflect.ValueOf(gossiper.RoutingTable).MapKeys()
	    nb_close_nodes := len(close_nodes)
	    nb_close_nodes_sent := len(gossiper.SentCloseNodes)

	    if(nb_close_nodes <= nb_close_nodes_sent) {
	    	return
	    }

	    closeNodesArray := []string{}
	    var n_str string

//	    for i := nb_close_nodes_sent; i < nb_close_nodes; i++ {
	    	for _,n := range close_nodes {
	    		n_str = n.Interface().(string)
	    		if(!contains(gossiper.SentCloseNodes, n_str)) {
	    			gossiper.SentCloseNodes = append(gossiper.SentCloseNodes, n_str)
	    			closeNodesArray = append(closeNodesArray, n_str)
	    		} 
	    	}
//	    }

//	    gossiper.LastCloseNodeSentIndex = nb_close_nodes - 1

	    json.Set("CloseNode", closeNodesArray)

		payload, err := json.MarshalJSON()
		isError(err)

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)

    } else {
        dest := r.FormValue("CloseNode")
        msg := r.FormValue("Message")

        // Write new private message to close node
        privateMsg := PrivateMessage{
			Origin : gossiper.Name,
			ID : 0,
			Text : msg,
			Destination : dest,
			HopLimit : 10,
		}

        address := getAddressFromRoutingTable(gossiper, dest)
		sendPrivateMsgToSpecificPeer(gossiper, privateMsg, address)
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

	var gossiper = NewGossiper("127.0.0.1:" + *UIPort, *gossipAddr, *name, *peers)

	if(*simple == true){
		// goroutines to listen on both ports simultaneously
		go simpleListenUIPort(gossiper)
		simpleListenGossipPort(gossiper)
	} else { 
		// goroutines to listen on both ports, run anti entropy and serve frontend simultaneously
		if(*rtimer != 0) {
			go generatePeriodicalRouteMessage(gossiper, *rtimer)
		}

		go listenUIPort(gossiper)
		listenGossipPort(gossiper)
		//go antiEntropy(gossiper)

		/*r := mux.NewRouter()

		r.HandleFunc("/id", func(w http.ResponseWriter, r *http.Request) {
		    IDHandler(gossiper, w, r)
		})
		r.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		    MessageHandler(gossiper, w, r)
		})
		r.HandleFunc("/node", func(w http.ResponseWriter, r *http.Request) {
		    NodeHandler(gossiper, w, r)
		})
		r.HandleFunc("/route", func(w http.ResponseWriter, r *http.Request) {
		    CloseNodeHandler(gossiper, w, r)
		})

	    http.ListenAndServe(":8080", handlers.CORS()(r))
	    */

	    // CHANGE SIMPLE IN GOSSIPER, CLIENT SHOULD SEND MESSAGE NOT RUMOR OR SIMPLE
	}
}