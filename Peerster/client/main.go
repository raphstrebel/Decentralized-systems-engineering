package main

import(
    "fmt"
    "flag"
    "net"
    "protobuf"
)

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

func isError(err error) {
    if err  != nil {
        fmt.Println("An error occurred : " , err)
    }
}

func sendPacket(packet GossipPacket, gossiperAddr string) {

    // Encode message
    packetBytes, err := protobuf.Encode(&packet)
    isError(err)
 
    // Start UDP connection
    ServerAddr, err := net.ResolveUDPAddr("udp", gossiperAddr)
    isError(err)
 
    conn, err := net.DialUDP("udp", nil, ServerAddr)
    isError(err)

    defer conn.Close()

    _,err = conn.Write(packetBytes)
    isError(err)
}

func main() {
    UIPort := flag.String("UIPort", "8080", "a string")
    dest := flag.String("dest", "", "a string")
    msg := flag.String("msg", "", "a string")
    flag.Parse()    
    gossiperAddr := "127.0.0.1:" + *UIPort

    if(*dest != "") {
        if(*msg != "") {
            privateMsg := &PrivateMessage{
                Origin : "client",
                ID : 0,
                Text : *msg,
                Destination : *dest,
                HopLimit : 10,
            }

            packet := GossipPacket{Private: privateMsg}
            sendPacket(packet, gossiperAddr)
        }
    } else {
        // Build a simple message : (for test_1_ring)
        //simpleMsg := &SimpleMessage{"client_name", gossiperAddr, *msg}
        //packet := &GossipPacket{Simple: simpleMsg}
        
            

        // Build a rumor message : (for test_2s_ring)
        var id uint32 = 1

        rumorMsg := &RumorMessage{
            Origin: "client_name", 
            ID: id,
            Text: *msg,
        }

        packet := GossipPacket{Rumor: rumorMsg}

        sendPacket(packet, gossiperAddr)
    }

}