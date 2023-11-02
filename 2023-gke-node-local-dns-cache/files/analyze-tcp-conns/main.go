// Reads data from Wireshark CSV export file located at tmp/packets.csv.
// You need the following columns in Wireshark for this program to read the fields correctly:
// "No.",          "Time",       "Source",   "Destination",   "Protocol",   "Length",       "Time",               "Source Port",   "Destination Port",   "Transaction ID",    "Name",            "Flags",    "Info"
//  Packet number   Packet time   Source IP   Destination IP   Protocol      Packet length   DNS Resolution time   Source port      Destination port      DNS Transaction ID   DNS Request name   TCP Flags   Info

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"time"
)

func main() {

	re, err := regexp.Compile(`(\d+:\d+:\d+)\..+`)
	if err != nil {
		fmt.Println(err)
	}

	timeLayout := "2006-01-02T15:04:05.99"

	// Open the CSV file
	// 0 - No.
	// 1 - Time
	// 2 - Source
	// 3 - Destination
	// 4 - Protocol
	// 5 - Length
	// 6 - Time (duration dns)
	// 7 - Source Port
	// 8 - Destination Port
	// 9 - DNS TX ID
	// 10 - Name (DNS domain name)
	// 11 - TCP Flags
	//			0x002 - SYN
	//			0x012 - SYN ACK
	//			0x010 - ACK
	//			0x011 - ACK FIN
	//			0x004 - RST
	//			0x018 - ACK PSH
	// 12 - Info

	file, err := os.Open("tmp/packets.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	data, err := reader.ReadAll()
	if err != nil {
		panic(err)
	}

	connectionState := make(map[string]string)
	connectionActive := make(map[string]bool)
	connectionLastPacketTime := make(map[string]time.Time)

	connectionCount := 0

	for i, row := range data {
		// Skip header row
		if i == 0 {
			continue
		}

		port := ""
		// If source port is not 53, use it, else use dest port
		if row[7] != "53" {
			port = row[7]
		} else {
			port = row[8]
		}

		if row[2] == "8.8.8.8" || row[2] == "8.8.4.4" || row[3] == "8.8.8.8" || row[3] == "8.8.4.4" {
			continue
		}

		fmt.Println()

		flags := map[string]string{
			"0x002": "SYN",
			"0x004": "RST",
			"0x011": "FINACK",
			"0x010": "ACK",
			"0x018": "PSHACK",
			"0x012": "SYNACK",
		}

		action := "none"

		// 0x002 - SYN tries to open connection. Be wary of frequent resends before connection is actually opened
		if row[11] == "0x002" {
			connectionState[port] = "SYN"
		}

		// 0x012 - SYN ACK opens connection.
		if row[11] == "0x012" {
			connectionState[port] = "SYNACK"
			action = "open"
		}

		// 0x004 - RST should close connection without any more back and forth
		if row[11] == "0x004" {
			connectionState[port] = "CLOSED"
			action = "close"
		}

		// 0x011 - ACK FIN
		if row[11] == "0x011" {
			connectionState[port] = "FINACK"
			action = "open"
		}

		// 0x010 - ACK || 0x018 - ACK PSH
		if row[11] == "0x010" || row[11] == "0x018" {
			// Reciving ACK after FINACK means connection is closed
			if connectionState[port] == "FINACK" {
				connectionState[port] = "CLOSED"
				action = "close"
			} else {
				connectionState[port] = "ACK"
				action = "open"
			}
		}

		result := "none"

		if action == "open" {
			if !connectionActive[port] {
				connectionActive[port] = true
				connectionCount++
				result = "adding"
			}
		} else if action == "close" {
			if connectionActive[port] {
				connectionActive[port] = false
				connectionCount--
				result = "removing"
			}
		}

		tParsed, _ := time.Parse(timeLayout, fmt.Sprintf("2023-10-10T%s", row[1]))
		if err != nil {
			fmt.Println(err)
		}

		timeSinceLastPacket := tParsed.Sub(connectionLastPacketTime[port])

		connectionLastPacketTime[port] = tParsed

		idleGroup := "fast"
		if timeSinceLastPacket > time.Duration(time.Millisecond*1000) && timeSinceLastPacket < time.Duration(time.Hour*200) {
			idleGroup = "slow"
		}

		tShort := re.FindStringSubmatch(row[1])

		fmt.Printf("Time: %v TimeShort: %v Port: %v Action: %v Result: %v Flags: %v IdleGroup: %v ConIdleTime: %v ActiveConnections: %v", row[1], tShort[1], port, action, result, flags[row[11]], idleGroup, timeSinceLastPacket, connectionCount)

	}

	fmt.Println()
}
