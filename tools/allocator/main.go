package main;

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
	IPV4Address string `json:"IPv4Address"`;
	SessionID   string `json:"SessionID"`;
};

var (
	ip string;
	certFile string;
	keyFile string;
);

func main () {
	args := os.Args;

	if len(args) == 1 {
		fmt.Println("Usage of the allocator tool (is highly recommended to have"+
					" kubectl on your $PATH)");
		fmt.Println("\t- allocate <build-id> <session-id> [tls-public] [tls-private]"+
					" # Initialize a server with the given paramaters (if tls certs"+
					" are not on the TLS_PUBLIC / TLS_PRIVATE env variables, please"+
					" provide them via argument)");
		fmt.Println("\t- list # Returns the available Game Servers");
	} else if strings.Compare(args[1], "allocate") == 0 {
		fmt.Println("Beginning the allocate process");

		cmd := exec.Command("kubectl","get","svc","-n","thundernetes-system","thundernetes-controller-manager",
			"-o","jsonpath='{.status.loadBalancer.ingress[0].ip}'");

		output, err := cmd.CombinedOutput();

		if err != nil {
			log.Fatal(string(output));
		}

		if len(output) < 3 { // basically if we don't have a valid IP
			ip = "http://127.0.0.1:5000/api/v1/allocate";
		} else {
			ip = string(output);
		}

		// get certificates to authenticate to operator API server
		if len(args) < 5 {
			certFile = os.Getenv("TLS_PUBLIC");
			keyFile = os.Getenv("TLS_PRIVATE");
		} else {
			certFile = args[4];
			keyFile = args[5];
		}
		
		cert, err := tls.LoadX509KeyPair(certFile, keyFile);

		if err != nil {
			log.Panic(err);
		}

		ar, err := allocate(ip, args[2], args[3], cert);

		if err != nil {
			log.Panic(err);
		}

		log.Println("IP address: "+ ar.IPV4Address+". Session ID: "+ar.SessionID);
		
	} else if strings.Compare(args[1], "list") == 0 {
		fmt.Println("Listing the available game servers");
		cmd := exec.Command("kubectl", "get", "gs");
		output, err := cmd.CombinedOutput();

		if err != nil {
			fmt.Println(string(output));
			log.Fatal("Error while fetching the servers: ", err);
			fmt.Println("Please, make sure you have your cluster configured properly");
		}

		fmt.Println(string(output));
	} else {
		fmt.Println("Sorry, but the commad "+args[1]+" is not recognized");
	}

	fmt.Println("\nThanks for using the thundernetes allocator tool");
	
}

func allocate(ip string, buildID string, sessionID string, cert tls.Certificate) (*AllocationResult, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig};
	client := &http.Client{Transport: transport};

	postBody, _ := json.Marshal(map[string]interface{}{
		"buildID":        buildID,
		"sessionID":      sessionID,
		"sessionCookie":  "randomCookie",
		"initialPlayers": []string{"player1", "player2"},
	});

	postBodyBytes := bytes.NewBuffer(postBody);
	resp, err := client.Post(ip+":5000/api/v1/allocate", "application/json", postBodyBytes);
	
	//Handle Error
	if err != nil {
		return nil, err;
	}

	defer resp.Body.Close();
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%d", resp.StatusCode);
	}

	//Read the response body
	body, err := ioutil.ReadAll(resp.Body);
	if err != nil {
		return nil, err;
	}

	ar := &AllocationResult{};
	json.Unmarshal(body, ar);

	if ar.IPV4Address == "" {
		return nil, fmt.Errorf("invalid IPV4Address %s", ar.IPV4Address);
	}

	if ar.SessionID != sessionID {
		return nil, fmt.Errorf("invalid SessionID %s", ar.SessionID);
	}

	return ar, nil;
}