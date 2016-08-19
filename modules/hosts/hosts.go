// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// Contributor: Julien Vehent jvehent@mozilla.com [:ulfr]

/* This is an example module. It doesn't do anything. It only serves as
a template for writing modules.
If you run it, it will return a JSON struct with the hostname and IPs
of the current endpoint. If you add flag `-p`, it will pretty print the
results.

 $ ./bin/linux/amd64/mig-agent-latest -p -m example <<< '{"class":"parameters", "parameters":{"gethostname": true, "getaddresses": true, "lookuphost": ["www.google.com"]}}'
 [info] using builtin conf
 hostname is fedbox2.jaffa.linuxwall.info
 address is 172.21.0.3/20
 address is fe80::8e70:5aff:fec8:be50/64
 lookedup host www.google.com has IP 74.125.196.106
 lookedup host www.google.com has IP 74.125.196.99
 lookedup host www.google.com has IP 74.125.196.104
 lookedup host www.google.com has IP 74.125.196.103
 lookedup host www.google.com has IP 74.125.196.105
 lookedup host www.google.com has IP 74.125.196.147
 lookedup host www.google.com has IP 2607:f8b0:4002:c07::69
 stat: 3 stuff found
*/
package hosts /* import "mig.ninja/mig/modules/hosts" */

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mig.ninja/mig/modules"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// An instance of this type will represent this module; it's possible to add
// additional data fields here, although that is rarely needed.
type module struct {
}

func (m *module) NewRun() modules.Runner {
	return new(run)
}

// init is called by the Go runtime at startup. We use this function to
// register the module in a global array of available modules, so the
// agent knows we exist
func init() {
	modules.Register("hosts", new(module))
}

type run struct {
	Parameters params
	Results    modules.Result
}

// a simple parameters structure, the format is arbitrary
type params struct {
	CheckDns    bool     `json:"checkdns"`
	CheckArp    bool     `json:"checkarp"`
	CheckHosts  bool     `json:"checkhosts"`
	SearchHosts []string `json:"searchhosts"`
	SearchIPs   []string `json:"searchips"`
}

type elements struct {
	// HostFound    string `json:"hostfound,omitempty"`
	DnsResults   []string `json:"dnsresults,omitempty"`
	HostsResults []string `json:"hostsresults,omitempty"`
	ArpResults   []string `json:"arpresults,omitempty"`
	//Hosts        map[string][]string `json:"hosts,omitempty"`
}

/* Statistic counters:
- DnsFound is the total targetHosts identified in the DNS cache
- HostsFound is the count of target hosts identified in the hosts file
- ArpIpsFound is the count of target IP addresses identified in the ARP cache
- Totalhits is the total number of checklist hits
- Exectim is the total runtime of all the searches
*/
type statistics struct {
	DnsFound    int           `json:"dnsfound"`
	HostsFound  int           `json:"hostsfound"`
	ArpIpsFound int           `json:"arpipsfound"`
	TotalHits   int           `json:"totalhits"`
	Exectime    time.Duration `json:"exectime"`
}

type ArpRecord struct {
	IPAddress  string
	MACAddress string
	Type       string
}

type HostRecord struct {
	IPAddress string
	Hostname  string
}

type DnsRecord struct {
	Type       string // 1, 5		, 12 , 28
	Record     string // A, CNAME , PTR, AAAA (A host records hold IP addresses)
	RecordName string // Hostname
	Section    string // Answer, Additional
}

// ValidateParameters *must* be implemented by a module. It provides a method
// to verify that the parameters passed to the module conform the expected format.
// It must return an error if the parameters do not validate.
func (r *run) ValidateParameters() (err error) {
	if r.Parameters.CheckHosts || r.Parameters.CheckDns {
		hosts := regexp.MustCompilePOSIX(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`)
		for _, host := range r.Parameters.SearchHosts {
			if !hosts.MatchString(host) {
				return fmt.Errorf("ValidateParameters: SearchHosts parameter is not a valid FQDN.")
			}
		}
	}

	if r.Parameters.CheckArp {
		ips := regexp.MustCompilePOSIX(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
		for _, ip := range r.Parameters.SearchIPs {
			if !ips.MatchString(ip) {
				return fmt.Errorf("ValidateParameters: SearchIPs parameter is not a valid IP address.")
			}
		}
	}

	// re, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	return
}

// Run *must* be implemented by a module. Its the function that executes the module.
// It must return a string of marshalled json that contains the results from the module.
// The code below provides a base module skeleton that can be reused in all modules.
func (r *run) Run(in io.Reader) (out string) {
	// a good way to handle execution failures is to catch panics and store
	// the panicked error into modules.Results.Errors, marshal that, and output
	// the JSON string back to the caller
	defer func() {
		if e := recover(); e != nil {
			r.Results.Errors = append(r.Results.Errors, fmt.Sprintf("%v", e))
			r.Results.Success = false
			buf, _ := json.Marshal(r.Results)
			out = string(buf[:])
		}
	}()

	// read module parameters from stdin
	err := modules.ReadInputParameters(in, &r.Parameters)
	if err != nil {
		panic(err)
	}
	// verify that the parameters we received are valid
	err = r.ValidateParameters()
	if err != nil {
		panic(err)
	}

	// start a goroutine that does some work and another one that looks
	// for an early stop signal
	moduleDone := make(chan bool)
	stop := make(chan bool)
	go r.doModuleStuff(&out, &moduleDone)
	go modules.WatchForStop(in, &stop)

	select {
	case <-moduleDone:
		return out
	case <-stop:
		panic("stop message received, terminating early")
	}
}

// doModuleStuff is an internal module function that does things specific to the
// module. There is no implementation requirement. It's good practice to have it
// return the JSON string Run() expects to return. We also make it return a boolean
// in the `moduleDone` channel to do flow control in Run().
func (r *run) doModuleStuff(out *string, moduleDone *chan bool) error {
	// fmt.Println("\nStart doModuleStuff...")
	var (
		el    elements
		stats statistics

		ar    ArpRecord
		dr    DnsRecord
		hr    HostRecord
		allar []string
		alldr []string
		allhr []string
	)

	// fmt.Println(r.Parameters)
	// hostResults := make([]string, 0, len(r.Parameters.SearchHosts))
	// dnsResults := make([]string, 0, len(r.Parameters.SearchHosts))
	// arpResults := make([]string, 0, len(r.Parameters.SearchIPs))
	// var (
	// hostResults []string
	// dnsResults []string
	// arpResults []string
	// )

	// el.Hosts = make(map[string][]string)
	t0 := time.Now()
	hostOS := runtime.GOOS
	stats.TotalHits = 0

	// Check arp cache for IP addresses
	if r.Parameters.CheckArp {
		// "arp -a" is universal for {linux/windows/darwin}
		cmdName := "arp"
		cmdArgs := []string{
			"-a",
		}
		cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
		if err != nil {
			panic(err)
		}

		arpMap := make(map[string]string)
		arpOutput := string(cmdOut)
		arpLines := strings.Split(arpOutput, "\n")

		for i := 3; i < len(arpLines); i++ {
			if len(arpLines[i]) > 1 {
				line := strings.Fields(arpLines[i])
				ar.IPAddress = line[0]
				ar.MACAddress = line[1]

				arpMap[ar.IPAddress] = ar.MACAddress
			}
			// allar = append(allar, ar)
		}

		/*
			Scan arp cache for each target ips provided in parameters
			- if found, append arp Record entry to results slice
		*/
		for _, targetIp := range r.Parameters.SearchIPs {
			if mac, ipFound := arpMap[targetIp]; ipFound {
				ar.IPAddress = targetIp
				ar.MACAddress = mac
				allar = append(allar, mac)

				stats.ArpIpsFound++
				stats.TotalHits++
			}
		}

		// Append results
		el.ArpResults = allar
	}

	// Check dns cache for target hosts
	if r.Parameters.CheckDns {

		recCount := 1
		if hostOS == "windows" {
			cmdName := "ipconfig"
			cmdArgs := []string{
				"/displaydns",
			}
			cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
			if err != nil {
				panic(err)
			}
			dnsMap := make(map[string]string)
			dnsOutput := string(cmdOut)
			dnsLines := strings.Split(dnsOutput, "\n")
			for i := 0; i < len(dnsLines); i++ {
				if strings.Index(dnsLines[i], ":") == -1 {
					continue
				}
				line := strings.Split(dnsLines[i], ":")
				dnsValue := strings.Replace(line[1], " ", "", -1)

				switch recCount {
				case 1:
					dr.RecordName = dnsValue
				case 2:
					dr.Type = dnsValue
				case 5:
					dr.Section = dnsValue
				case 6:
					dr.Record = dnsValue
				}

				recCount++
				if recCount > 6 {
					recCount = 1
					dnsMap[dr.Record] = dr.RecordName
				}
			}

			/*
				Scan 'hosts' file for each target host provided in parameters
				- if found, append HostRecord entry to results slice
			*/
			for _, targetHost := range r.Parameters.SearchHosts {
				if ip, hostFound := dnsMap[targetHost]; hostFound {
					// hr.Hostname = targetHost
					// hr.IPAddress = ip
					// dnsResults = append(dnsResults, ip)
					alldr = append(alldr, ip)
					stats.DnsFound++
					stats.TotalHits++
				}
			}
		}
		el.DnsResults = alldr

	}

	// Check 'hosts' file for target hostnames
	if r.Parameters.CheckHosts {
		var hostPath string
		hostMap := make(map[string]string)
		var HostsOutput string

		if hostOS == "windows" {
			hostPath = "C:\\Windows\\System32\\drivers\\etc\\hosts"
			hostfile, err := ioutil.ReadFile(hostPath)
			HostsOutput = fmt.Sprintf("%s", hostfile)
			if err != nil {
				panic(err)
			}
		} else {
			hostPath = "/etc/hosts"
			cmdName := "cat"
			cmdArgs := []string{
				hostPath,
			}

			cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
			if err != nil {
				panic(err)
			}

			HostsOutput = string(cmdOut)
		}

		hostLines := strings.Split(HostsOutput, "\n")
		for i := 0; i < len(hostLines); i++ {
			if strings.HasPrefix(hostLines[i], "#") || len(hostLines[i]) < 2 {
				continue
			}

			line := strings.Fields(hostLines[i])
			hr.IPAddress = line[0]
			hr.Hostname = line[1]
			hostMap[hr.Hostname] = hr.IPAddress
		}

		/*
			Scan 'hosts' file for each target host provided in parameters
			- if found, append HostRecord entry to results slice
		*/
		for _, targetHost := range r.Parameters.SearchHosts {
			if ip, hostFound := hostMap[targetHost]; hostFound {
				hr.Hostname = targetHost
				hr.IPAddress = ip

				allhr = append(allhr, ip)
				stats.HostsFound++
				stats.TotalHits++
			}
		}
		el.HostsResults = allhr

	}

	stats.Exectime = time.Now().Sub(t0)

	// marshal the results into a json string
	*out = r.buildResults(el, stats)
	*moduleDone <- true
	return nil
}

// buildResults takes the results found by the module, as well as statistics,
// and puts all that into a JSON string. It also takes care of setting the
// success and foundanything flags.
func (r *run) buildResults(el elements, stats statistics) string {
	if len(r.Results.Errors) == 0 {
		r.Results.Success = true
	}
	r.Results.Elements = el
	r.Results.Statistics = stats
	if stats.TotalHits > 0 {
		r.Results.FoundAnything = true
	}
	jsonOutput, err := json.Marshal(r.Results)
	if err != nil {
		panic(err)
	}
	return string(jsonOutput[:])
}

// PrintResults() is an *optional* method that returns results in a human-readable format.
// if matchOnly is set, only results that have at least one match are returned.
// If matchOnly is not set, all results are returned, along with errors and statistics.
func (r *run) PrintResults(result modules.Result, matchOnly bool) (prints []string, err error) {
	var (
		el    elements
		stats statistics
	)
	err = result.GetElements(&el)
	if err != nil {
		panic(err)
	}

	prints = append(prints, fmt.Sprintf("\n-----------------\n     Hosts Results           \n------------------"))

	for _, host := range el.HostsResults {
		prints = append(prints, fmt.Sprintf("Found Host Entry:  %s", host))
	}

	for _, dns := range el.DnsResults {
		prints = append(prints, fmt.Sprintf("Found DNS Entry:  %s", dns))
	}
	for _, ip := range el.ArpResults {
		// for _, addr := range addrs {
		prints = append(prints, fmt.Sprintf("Found IP Address:  %s", ip))
		// }
	}

	for _, e := range result.Errors {
		prints = append(prints, fmt.Sprintf("error: %v", e))
	}
	err = result.GetStatistics(&stats)
	if err != nil {
		panic(err)
	}

	prints = append(prints, fmt.Sprintf("DNS  : Total of %d entries found", stats.DnsFound))
	prints = append(prints, fmt.Sprintf("ARP  : Total of %d entries found", stats.ArpIpsFound))
	prints = append(prints, fmt.Sprintf("Hosts: Total of %d entries found", stats.HostsFound))
	prints = append(prints, fmt.Sprintf("stats: Total of %d entries found", stats.TotalHits))
	prints = append(prints, fmt.Sprintf("Time : %v", stats.Exectime))
	return
}
