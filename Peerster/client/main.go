package main

import(
    "fmt"
    "flag"
    "net"
    "protobuf"
)

type ClientPacket struct {
    Message *NormalMessage
    Private *PrivateMessage
}

type NormalMessage struct {
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

func sendPacket(packet ClientPacket, gossiperAddr string) {

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
    file := flag.String("file", "", "a string")
    flag.Parse()    

    gossiperAddr := "127.0.0.1:" + *UIPort

    fmt.Println(file)

    if(*dest != "") {
        if(*msg != "") {
            privateMsg := &PrivateMessage{
                Origin : "client",
                ID : 0,
                Text : *msg,
                Destination : *dest,
                HopLimit : 10,
            }

            packet := ClientPacket{Private: privateMsg}
            sendPacket(packet, gossiperAddr)
        }
    } else {
        
        message := &NormalMessage{
            Text: *msg,
        }

        packet := ClientPacket{Message: message}

        sendPacket(packet, gossiperAddr)
    }

}