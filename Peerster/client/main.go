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
    File *FileMessage
    Request *FileRequestMessage
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

type FileMessage struct {
    FileName string
}

type FileRequestMessage struct {
    FileName string
    Destination string
    Request string
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
    UIPort := flag.String("UIPort", "8080", "the UIPort as string")
    dest := flag.String("dest", "", "a destination node as string")
    msg := flag.String("msg", "", "a message as string")
    file := flag.String("file", "", "a file as string")
    request := flag.String("request", "", "a hexadecimal metahash as string")
    flag.Parse() 

    gossiperAddr := "127.0.0.1:" + *UIPort

    if(*dest != "") {
        // Send a private message
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

        // request is the metahash of file we want, file is the name of the file we want to save requested as
        if(*request != "") { 
            if(*file != "") {
                fileMessage := &FileRequestMessage{
                    FileName: *file,
                    Destination: *dest,
                    Request: *request,
                }

                packet := ClientPacket{Request: fileMessage}
                sendPacket(packet, gossiperAddr)
            }
        }
    } else if(*file != "") { 
        if(*file != "") {
            // check if the file exists ?
            fileMessage := &FileMessage{
                FileName: *file,
            }

            packet := ClientPacket{File: fileMessage}
            sendPacket(packet, gossiperAddr)
        }

    } else { // dest is nil, so send a normal message
        
        message := &NormalMessage{
            Text: *msg,
        }

        packet := ClientPacket{Message: message}
        sendPacket(packet, gossiperAddr)
    }
}