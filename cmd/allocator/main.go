package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
);

type AllocationResult struct {
	IPV4Address string `json:"IPv4Address"`
	SessionID   string `json:"SessionID"`
};

var (
	ip string
	certFile string
	keyFile string
	tlsSet bool
	ar *AllocationResult
);

func main () {
	args := os.Args

	if len(args) == 1 {
		fmt.Println("Usage of the allocator tool (is required to have"+
					" kubectl on your $PATH)")
		fmt.Println("\t- allocate <build-id> <session-id> [tls-public] [tls-private]"+
					" # Initialize a server with the given paramaters (if tls certs"+
					" are not on the TLS_PUBLIC / TLS_PRIVATE env variables, please"+
					" provide them via argument)")
		fmt.Println("\t- list # Returns the available Game Servers")
	} else if strings.Compare(args[1], "allocate") == 0 {
		fmt.Println("Beginning the allocate process")

		cmd := exec.Command("kubectl","get","svc","-n","thundernetes-system","thundernetes-controller-manager",
			"-o","jsonpath='{.status.loadBalancer.ingress[0].ip}'")

		output, err := cmd.CombinedOutput()

		if err != nil {
			log.Println("Is required to have kubectl on your $PATH")
			log.Fatal(string(output))
			
		}

		if len(args) < 5 { // if no more arguments are provided
			if certFile == "" || keyFile == "" { // If the env vars are not set
				tlsSet = false
			} else { // the env vars are set 
				tlsSet = true
			}
		} else { //  all the arguments are provided
			tlsSet = true
		}

		if len(output) < 3 { // basically if we don't have a valid IP
			if tlsSet == true {
				ip = "https://127.0.0.1"
				cert, err := tls.LoadX509KeyPair(certFile, keyFile)
				ar, err = allocateTls(ip, args[2], args[3], cert)

				if err != nil {
					log.Panic(err)
				}		
			} else {
				ip = "http://127.0.0.1"
				ar, err = allocateNoTls(ip, args[2], args[3])

				if err != nil {
					log.Panic(err)
				}		
			}
		} else { //  if we retrieve the ip correctly
			ip = string(output)

			if tlsSet == true {
				cert, err := tls.LoadX509KeyPair(certFile, keyFile)
				ar, err = allocateTls(ip, args[2], args[3], cert)

				if err != nil {
					log.Panic(err)
				}
			} else {
				ar, err = allocateNoTls(ip, args[2], args[3])

				if err != nil {
					log.Panic(err)
				}
			}
		}

		log.Println("IP address: "+ ar.IPV4Address+". Session ID: "+ar.SessionID);
		
	} else if strings.Compare(args[1], "list") == 0 {
		fmt.Println("Listing the available game servers")
		cmd := exec.Command("kubectl", "get", "gs")
		output, err := cmd.CombinedOutput()

		if err != nil {
			fmt.Println(string(output))
			log.Fatal("Error while fetching the servers: ", err)
			log.Println("It is required to have kubectl on your $PATH")
			fmt.Println("Please, make sure you have your cluster configured properly")
		}

		fmt.Println(string(output))
	} else {
		fmt.Println("Sorry, but the commad "+args[1]+" is not recognized")
	}

	fmt.Println("\nThanks for using the thundernetes allocator tool")
	
}

func allocateTls(ip string, buildID string, sessionID string, cert tls.Certificate) (*AllocationResult, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	postBody, _ := json.Marshal(map[string]interface{}{
		"buildID":        buildID,
		"sessionID":      sessionID,
		"sessionCookie":  "coolRandomCookie",
		"initialPlayers": []string{"player1", "player2"},
	})

	postBodyBytes := bytes.NewBuffer(postBody)
	resp, err := client.Post(ip+":5000/api/v1/allocate", "application/json", postBodyBytes)
	
	//Handle Error
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%d", resp.StatusCode)
	}

	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ar := &AllocationResult{}
	json.Unmarshal(body, ar)

	if ar.IPV4Address == "" {
		return nil, fmt.Errorf("invalid IPV4Address %s", ar.IPV4Address)
	}

	if ar.SessionID != sessionID {
		return nil, fmt.Errorf("invalid SessionID %s", ar.SessionID)
	}

	return ar, nil
}

func allocateNoTls(ip string, buildID string, sessionID string) (*AllocationResult, error) {

	transport := &http.Transport{}
	client := &http.Client{Transport: transport}

	postBody, _ := json.Marshal(map[string]interface{}{
		"buildID":        buildID,
		"sessionID":      sessionID,
		"sessionCookie":  "coolRandomCookie",
		"initialPlayers": []string{"player1", "player2"},
	})

	postBodyBytes := bytes.NewBuffer(postBody)
	resp, err := client.Post(ip+":5000/api/v1/allocate", "application/json", postBodyBytes)
	
	//Handle Error
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%d", resp.StatusCode)
	}

	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ar := &AllocationResult{}
	json.Unmarshal(body, ar)

	if ar.IPV4Address == "" {
		return nil, fmt.Errorf("invalid IPV4Address %s", ar.IPV4Address)
	}

	if ar.SessionID != sessionID {
		return nil, fmt.Errorf("invalid SessionID %s", ar.SessionID)
	}

	return ar, nil
}