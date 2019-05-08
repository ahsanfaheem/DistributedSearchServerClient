
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"
	"bytes"
	"strings"
	"bufio"
	"strconv"
)
type Request struct{
	password string
	channel chan bool
}
var requests = make(map[int](Request))

func handleSearch() {
	ln, err := net.Listen("tcp", ":7000")
	if err != nil {
		fmt.Println(err)
	} else {
		defer ln.Close()
		for {
			c, err := ln.Accept()
			if err != nil {
				fmt.Println(err)
				continue
			} else {
				fmt.Printf("connected")
				defer c.Close()
				var b = make([]byte, 3096)
				c.Read(b)
				fmt.Println(b)
			}
		}
	}
}
func handleHeartBeat(con net.Conn, a chan<- string) {
	defer func() { a <- "dead" }()
	for {
		_, err := con.Write([]byte("[HEARBEAT]"))
		if err != nil {
			fmt.Printf("connection error")
			return
		}
		fmt.Printf("heart beat sent")
		time.Sleep(2 * time.Second)
	}
}
func search(rId string, slaveId string,password string, files []string,con net.Conn,reqID int,chanNo int){
	var found=false
	for _,v:=range(files){
		//b, _:=ioutil.ReadFile("./passwords/"+v)
	
		//contentString:=string(b)
		fmt.Println("Printing contents of: "+v)
		//fmt.Println(contentString)
		file, err:=os.Open("./passwords/"+v)
		if err!=nil{
			fmt.Println(err)
		}
		defer file.Close()
		scanner:=bufio.NewScanner(file)
		for scanner.Scan(){
			select {
				case <-requests[reqID].channel:
					fmt.Println("Terminating: "+rId)
					return
				default:
					fileLine:=scanner.Text()
					//fmt.Println(fileLine)
					if(fileLine==password){
						fmt.Println("password found!!!!!!!!!!")
						found=true
						con.Write([]byte("[RESULT]:"+rId+":[SLAVEID]:"+slaveId+":FOUND"))
						break
					}
			}
			
		}
	/*	if(strings.Contains(contentString,password)){
			fmt.Println("password found!!!!!!!!!!")
			found=true
			con.Write([]byte("[RESULT]:"+rId+":[SLAVEID]:"+slaveId+":FOUND"))
			break
		}*/
		if(found){
			break
		}
	}
	if(!found){
		fmt.Println("Your password is safe!!!!")
		con.Write([]byte("[RESULT]:"+rId+":[SLAVEID]:"+slaveId+":NOT-FOUND"))
	}


}
func receiveRequests(con net.Conn){
	for{
		request := make([]byte, 800)
		con.Read(request)
		request=bytes.Trim(request,"\x00")
		requestString:=strings.TrimSpace(string(request))
		fmt.Println(requestString)
		fmt.Println(requestString[:11])

		if(strings.Contains(requestString,"SID")){
			filesToSearch:=strings.Split(requestString,",")
			//filesToSearch=append(filesToSearch[:1],filesToSearch[2:]...)
			otherInfo:=strings.Split(filesToSearch[0],":")
			filesToSearch=append(filesToSearch[1:1],filesToSearch[2:]...)

			requestId:=otherInfo[1]
			passwordString:=otherInfo[2]
			slaveId:=otherInfo[4]
			fmt.Println("Id:"+requestId)
			fmt.Println("password:"+passwordString)
			fmt.Println("slave id: "+slaveId)
			fmt.Printf("%v", filesToSearch)
			reqIDInt,_:=strconv.Atoi(requestId)
			
			_,ok:=requests[reqIDInt]
			if(!ok){
				req := Request{
					password:passwordString}
				req.channel=make(chan bool,0)
				requests[reqIDInt]=req
			}
			
			fmt.Println("starting search...")
			go search(requestId,slaveId, passwordString,filesToSearch,con,reqIDInt,len(requests[reqIDInt].channel)-1)

			}else if(requestString[:11]=="[COMPLETED]"){
				wordsReceived:=strings.Split(requestString,":")
				reqID:=wordsReceived[1]
				reqIDInt,_:=strconv.Atoi(reqID)
		
					requests[reqIDInt].channel<-true
				
			}
					
	}

}
func main() {
	alive := make(chan string)
	fmt.Printf("connecting to: " + os.Args[1])
	conn, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		log.Printf("connection error")
	} else {
		log.Printf("connected")
		defer conn.Close()
		files, err := ioutil.ReadDir("./passwords")
		if err != nil {
			log.Fatal(err)
		}
		filesNames := "[DIRSTRUCT]:"
		for _, file := range files {
			fmt.Println(file.Name())
			filesNames += file.Name() + ":"
		}
		conn.Write([]byte(filesNames))
		go handleHeartBeat(conn, alive)
		go receiveRequests(conn)
		s := <-alive
		fmt.Printf(s)
		//return
	}
}
