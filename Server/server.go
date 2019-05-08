package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"bytes"
	"html/template"
	"math/rand"
	"strconv"
	"time"
)

//Slave is a struct for slave
type Slave struct {
	id int
	conn  net.Conn
	files map[string]struct{}
	ch    chan string
	isBusy bool
	isTerminated bool
	currentFilesUsed []string
	currentRequestId int
}
type Request struct{
	slavesCount int
	slavesId []int
	password string
	channel chan string
	isCompleted bool
	
}
var slaves = make([]Slave,0)
//var request = make(map[int](chan string))
var requests = make(map[int](Request))

var uniqueFiles=make(map[string]struct{})
func scheduleSearch(password string,rId int) (int, []int){
	var tasks=make(map[int]string)
	for k,_:=range(uniqueFiles){
		index:=findServerWithChunk(k)
		//fmt.Println(index)
		if(index!=-1){
			if(!slaves[index].isBusy){
				slaves[index].isBusy=true
			}
			_, slaveExists :=tasks[index]
			if(!slaveExists){
				tasks[index]=",";
				slaves[index].currentFilesUsed=make([]string,0);
			}
			//fmt.Println("K: "+k)
			tasks[index]=tasks[index]+","+k
			slaves[index].currentFilesUsed=append(slaves[index].currentFilesUsed,k)
			
		}
	}
	searchStringPrefix:="SID:"+strconv.Itoa(rId)+":"+password
	slavesGivenTask:=make([]int,0)
	for k,v:=range tasks{
		searchString:=searchStringPrefix+":SLID:"+strconv.Itoa(k)+v
		slaves[k].currentRequestId=rId
		slavesGivenTask=append(slavesGivenTask,k)
		slaves[k].conn.Write([]byte(searchString))
		//fmt.Printf("current: %v ",slaves[k].currentFilesUsed)
	}
	return len(tasks),slavesGivenTask

}
func reScheduleTask(reqID int, fileNames []string){
	fmt.Println("Re scheduling...")
	var tasks=make(map[int]string)
	for _,v:=range(fileNames){
		index:=findServerWithChunk(v)
		//fmt.Println(index)
		if(index!=-1){
			
			if(!slaves[index].isBusy){
				slaves[index].isBusy=true
				
			}else{
				
			}
			_, slaveExists :=tasks[index]
			if(!slaveExists){
				tasks[index]=",";
				slaves[index].currentFilesUsed=make([]string,0);
			}
			//fmt.Println("K: "+v  )
			tasks[index]=tasks[index]+","+v
			slaves[index].currentFilesUsed=append(slaves[index].currentFilesUsed,v)
			
		}else{
			fmt.Println("Couldn't find any slave with chunk: "+v)
		}
		
	}
	searchStringPrefix:="SID:"+strconv.Itoa(reqID)+":"+requests[reqID].password
	for k,v:=range tasks{
		searchString:=searchStringPrefix+":SLID:"+strconv.Itoa(k)+v
		slaves[k].conn.Write([]byte(searchString))
		//fmt.Printf("current: %v ",slaves[k].currentFilesUsed)
	}
}
func findServerWithChunk(chunk string) int{
	busySlavesWithChunks:=make([]int,0)
	for i,slave:=range slaves{
		_, chunkExists :=slave.files[chunk]
		if(!slave.isTerminated && !slave.isBusy && chunkExists){
				//fmt.Println("Found free slave: with chunk "+chunk)
				//slaves[i].isBusy=true
				return i
		}else if (!slave.isTerminated && chunkExists){
			busySlavesWithChunks=append(busySlavesWithChunks,i)
		}
	}
	if(len(busySlavesWithChunks)>0){
		return busySlavesWithChunks[rand.Int31n(int32(len(busySlavesWithChunks)))]
	}else{
		return -1
		}
	}
func requestHandler(w http.ResponseWriter, r *http.Request) {
	passwordToSearch:=string(r.FormValue("passwordAsked"))
	fmt.Println("New Client Request: "+passwordToSearch)
	if(passwordToSearch==""){
		t, _:= template.ParseFiles("index.html")
		t.Execute(w,nil)
		
	}else{
		req := Request{
			slavesCount:  0}
		req.channel=make(chan string)
		requestID:=len(requests)
		
		numberOfSlaves,slavesForTasks:=scheduleSearch(passwordToSearch,requestID)
		req.slavesCount=numberOfSlaves
		req.password=passwordToSearch
		req.isCompleted=false
		req.slavesId=slavesForTasks
		requests[requestID]=req
		count:=0
		isCracked:=false
		for count<numberOfSlaves{
			 count+=1
			 res:=<-requests[requestID].channel
			 if(res=="FOUND"){
				 isCracked=true
				 break
			 }
		}
		if(isCracked){
			fmt.Println("Password Found")			
			t, _:= template.ParseFiles("found.html")
			t.Execute(w,nil)
		}else{
			fmt.Println("Password Not Found")
			t, _:= template.ParseFiles("notfound.html")
			t.Execute(w,nil)
		}
		req=requests[requestID]
		req.isCompleted=true
		requests[requestID]=req
	}
}
func handleSlaves(addchan <-chan Slave, rmchan <-chan Slave) {
	for {
		select {
		case slave := <-addchan:
			fmt.Println("-> New slave connected")
			slaves=append(slaves,slave)
			for k :=range slave.files{
				_, ok :=uniqueFiles[k]
				if !ok{
					fmt.Println("-> Adding files to list of Unique Files")
					uniqueFiles[k]=struct{}{}
				}
			}
			//scheduleSearch("abc")
			//slaves["Slave:"+string(len(slaves))] = slave
					
		case slave := <-rmchan:
			log.Printf("Client disconnects: %v\n", slave.conn)
			//delete(slaves, slave.conn)
		}
	}
}
func handleConnections(c net.Conn, addchan chan<- Slave, rmchan chan<- Slave) {

	//welcoming new slaves
	b := make([]byte, 2048)
	c.Read(b)
	b=bytes.Trim(b,"\x00")
	slavaDataString:=strings.TrimSpace(string(b))
	//fmt.Println(slavaDataString)
	
	slave := Slave{
		conn:  c,
		ch:    make(chan string),
		
		//isBusy: false
	}

	slave.files=make(map[string]struct{})
	if (strings.Contains(slavaDataString,"[DIRSTRUCT]")){
		fmt.Println("-> Directory structure Received")
		fileNames:=strings.Split(slavaDataString,":")
		fileNames=append(fileNames[:0],fileNames[1:len(fileNames)-1]...)
		for _, name := range fileNames{
			fmt.Println(name)
			slave.files[name]=struct{}{}
		}
		if len(slave.files) == 0 {
			fmt.Printf("No chunk found")
			io.WriteString(c, "Slave has no password chunks\n")
			return
		}
		slave.conn.SetReadDeadline(time.Now().Add(7*time.Second))
		slave.id=len(slaves)
		addchan <- slave
		for {
			slaveDataReceived := make([]byte, 200)
			_,err:=slave.conn.Read(slaveDataReceived)
			if(err!=nil){
				if(strings.Contains(err.Error(),"An existing connection was forcibly")){
					fmt.Println("Slave with ID: "+strconv.Itoa(slave.id)+" disconnected")
					slaves[slave.id].isTerminated=true
					if(slaves[slave.id].isBusy==true){
						
							reScheduleTask(slaves[slave.id].currentRequestId,slaves[slave.id].currentFilesUsed)	
						
					}
				}
				break
			}else{
				slaveDataReceived=bytes.Trim(slaveDataReceived,"\x00")
				slaveDataReceivedString:=strings.TrimSpace(string(slaveDataReceived))
				fmt.Printf(slaveDataReceivedString)
				if(strings.Contains(slaveDataReceivedString,"[RESULT]")){
					infos:=strings.Split(slaveDataReceivedString,":")
					rID:=infos[1]
					slaveID:=infos[3]
					result:=infos[4]
					/*fmt.Println("rid: "+rID)
					fmt.Println("slid: "+slaveID)
					fmt.Println("result: "+result)*/
					rIDint,_:=strconv.Atoi(rID)
					slaveIDint,_:=strconv.Atoi(slaveID)
					slaves[slaveIDint].isBusy=false
					
					if(requests[rIDint].isCompleted==false){
						requests[rIDint].channel<-result
					}
					if(result=="FOUND"){
						fmt.Println("found!!!!!!!!!!!!!!!!!!!!")
						for _,v :=range(requests[rIDint].slavesId){
							if(v!=slaveIDint){
								if(slaves[v].isBusy){
									fmt.Println("Other slaves seraching: "+strconv.Itoa(v))
									toSend:="[COMPLETED]:"+rID
									slaves[v].isBusy=false
									slaves[v].conn.Write([]byte(toSend))
							
								}
							}
						}

					}
					
				}
					slave.conn.SetReadDeadline(time.Now().Add(7*time.Second))
				
			}
			//time.Sleep(2 * time.Second)
		}
	}


	// Register user, our messageHandler is waiting on this channel
	//it populates the map
	

}
func webPageFunc(){
	http.HandleFunc("/", requestHandler)
	http.ListenAndServe(":8000", nil)
}
func main() {
	//	slaves := make([]Slave, 6)
	go webPageFunc()
	
	
	ln, err := net.Listen("tcp", "localhost:6000")
	//searchchan := make(chan string)
	addchan := make(chan Slave)
	rmchan := make(chan Slave)
	go handleSlaves(addchan, rmchan)

	if err != nil {
		fmt.Println(err)
	} else {
		defer ln.Close()
		for {
			fmt.Println("Waiting for slave...")
			c, err := ln.Accept()
			if err != nil {
				fmt.Println(err)
				continue
			} else {
				fmt.Printf("Connected with a new slave...")
				//	defer c.Close()
				go handleConnections(c, addchan, rmchan)
			}
		}
	}

	

}
