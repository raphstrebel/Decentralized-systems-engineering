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
    Search *FileSearchMessage
    SearchDownload *FileDownloadMessage
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

type FileSearchMessage struct {
    Keywords string
    Budget uint64
}

type FileDownloadMessage struct {
    Filename string
    Metahash string
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
    keywords := flag.String("keywords", "", "comma-separated list of keywords as string")
    budget := flag.Uint64("budget", 0, "an int >= 0")
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

        // Homework 3, we should be able to download a file just by putting in the file and the request fields (no dest)
        if(*request != "") {
            // check if the file exists ?
            fileDownloadMessage := &FileDownloadMessage{
                Filename: *file,
                Metahash: *request,
            }

            packet := ClientPacket{SearchDownload: fileDownloadMessage}
            sendPacket(packet, gossiperAddr)
        } else {
            // check if the file exists ?
            fileMessage := &FileMessage{
                FileName: *file,
            }

            packet := ClientPacket{File: fileMessage}
            sendPacket(packet, gossiperAddr)
        }
    } else if(*keywords != "") {
        
        // WHEN SHOULD WE CHECK IF THE BUDGET HAS BEEN GIVEN ?
        fileSearchMessage := &FileSearchMessage{
            Keywords: *keywords,
            Budget: *budget,
        }

        packet := ClientPacket{Search: fileSearchMessage}
        sendPacket(packet, gossiperAddr)
    } else if(*msg != "") { // dest and file is nil, so send a normal message
        
        message := &NormalMessage{
            Text: *msg,
        }

        packet := ClientPacket{Message: message}
        sendPacket(packet, gossiperAddr)
    } 
}