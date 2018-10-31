package main

import(
	"fmt"
	"net"
	"protobuf"
)

func simpleListenUIPort(gossiper *Gossiper) {

	defer gossiper.UIPortConn.Close()
 
    packetBytes := make([]byte, 1024)
 
    for true {

        _,_,err := gossiper.UIPortConn.ReadFromUDP(packetBytes)
        isError(err)

        receivedPkt := &ClientPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

        if(receivedPkt.Message != nil) {

			msg :=  receivedPkt.Message.Text

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