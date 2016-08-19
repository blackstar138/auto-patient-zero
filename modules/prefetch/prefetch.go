/*
	@TODO: GPL License 2016
*/

/*
	@TODO: Description of Prefetch module: 	i) Purpose
										  	ii) Parameters
										  	iii) Output
										  	iv) Example json/input json
										  	v) How to run it / example output

If you run it, it will return a JSON struct with the hostname and IPs
of the current endpoint. If you add flag `-p`, it will pretty print the
results.

 $ ./bin/linux/amd64/mig-agent-latest -p -m example <<< '{"class":"parameters", "parameters":{"gethostname": true, "getaddresses": true, "lookuphost": ["www.google.com"]}}'
 [info] using builtin conf
 hostname is fedbox2.jaffa.linuxwall.info
 address is 172.21.0.3/20
 address is fe80::8e70:5aff:fec8:be50/64
 lookedup host www.google.com has IP 74.125.196.106
 stat: 3 stuff found
*/
package prefetch /* import "mig.ninja/mig/modules/prefetch" */

import (
	"encoding/json"
	"fmt"
	"io"
	"mig.ninja/mig/modules"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

/*
	An instance of this type will represent this module; it's possible to add additional data fields here,
	although that is rarely needed.
*/
type module struct {
}

func (m *module) NewRun() modules.Runner {
	return new(run)
}

/*
	init is called by the Go runtime at startup. We use this function to register the module in a
	global array of available modules, so the agent knows we exist
*/
func init() {
	modules.Register("prefetch", new(module))
}

type run struct {
	Parameters params
	Results    modules.Result
}

/*
	- ParseDLL : Should the program only return run count and execution date,
				 or resource and directory string lists
	- SearchExe: Array of executable file names to search for
	- SearchDLL: Array of referenced libraries to search for. Requires ParseDLL=true
	- GetLastDate: Specify whether earliest or latest run date should be returned
	- Debug: Enable debug print statements (not supported right now)
*/
type params struct {
	ParseDLL    bool     `json:"parsedll"`
	SearchExe   []string `json:"searchexe"`
	SearchDLL   []string `json:"searchdll"`
	GetLastDate bool     `json:"getlastdate"`
	Debug       bool     `json:"debug"`
}

type PrefetchRecord struct {
	ExeName          string     `json:"exename,omitempty"`
	RunCount         string     `json:"runcount,omitempty"`
	Volume           volumeInfo `json:"volume,omitempty"`
	DirectoryStrings []string   `json:"directorystrings,omitempty"`
	ResourcesLoaded  []string   `json:"resourcesloaded,omitempty"`
	DateExecuted     string     `json:"dateexecuted,omitempty"`
}

type volumeInfo struct {
	VolumeName   string `json:"volumename,omitempty"`
	CreationDate string `json:"creationdate,omitempty"`
	Serial       string `json:"serial,omitempty"`
}

type PrefetchResult struct {
	ExeName  string `json:"exename,omitempty"`
	DLLName  string `json:"dllname,omitempty"`
	ExecDate string `json:"execdate,omitempty"` // @TODO: Change type to time.Time + include parsing
	RunCount string `json:"runcount,omitempty"`
}

/*
	Results returned as slice of PrefetchResult objects, containing:
	i) pgm name (ii) dll name (iii) execution date (iv) run count
*/
type elements struct {
	prefetch []PrefetchResult `json:"prefetchresults,omitempty"`
}

/* Statistic counters:
- DLLsFound is the total DLL libraries identified in the prefetch file
- ExesFound is the count of target executables identified in prefetch file
- Totalhits is the total number of checklist hits
- Exectim is the total runtime of all the searches
*/
type statistics struct {
	DLLsFound   int           `json:"dllsfound"`
	ExesFound   int           `json:"exesfound"`
	NumPrefetch int           `json:"numprefetch"`
	TotalHits   int           `json:"totalhits"`
	Exectime    time.Duration `json:"exectime"`
}

/*
	@TODO: Create some parameter validation

	ValidateParameters *must* be implemented by a module. It provides a method to verify that the parameters
	passed to the module conform the expected format. It must return an error if the parameters do not validate.
*/
func (r *run) ValidateParameters() (err error) {
	// if r.Parameters.CheckHosts || r.Parameters.CheckDns {
	// 	hosts := regexp.MustCompilePOSIX(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`)
	// 	for _, host := range r.Parameters.SearchHosts {
	// 		if !hosts.MatchString(host) {
	// 			return fmt.Errorf("ValidateParameters: SearchHosts parameter is not a valid FQDN.")
	// 		}
	// 	}
	// }

	// re, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	return
}

/*
	Run *must* be implemented by a module. Its the function that executes the module. It must return a string of
	marshalled json that contains the results from the module. The code below provides a base module skeleton that
	can be reused in all modules.
*/
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

/* doModuleStuff is an internal module function that does things specific to the module. There is no implementation requirement.
   It's good practice to have it return the JSON string Run() expects to return. We also make it return a boolean in the `moduleDone`
   channel to do flow control in Run().
*/
func (r *run) doModuleStuff(out *string, moduleDone *chan bool) error {
	if r.Parameters.Debug {
		// fmt.Println("Params: \n------------------------")
		// fmt.Println("ParseDLL: ", r.Parameters.ParseDLL)
		// fmt.Println("Target Exe's: ", r.Parameters.SearchExe)
		// fmt.Println("Target DLL's: ", r.Parameters.SearchDLL)
	}
	timeStart := time.Now()
	var (
		el    elements
		stats statistics
		pr    PrefetchRecord
		allpr []PrefetchRecord

		endOfRecord bool
		numPrefetch int
		exeName     string
		runCount    string
		dirStrings  []string
		resList     []string
		returnDate  string
		execDate    string
		execTime    string
		volInfo     volumeInfo
	)

	stats.TotalHits = 0 // counter for found entries

	if runtime.GOOS != "windows" {
		//return // prefetch Module only for Windows OS machines
		return fmt.Errorf("Error: Prefetch Searching can only be run on a Windows environment.")
	}

	/*
		- Configure Prefetch Parsing Commands
		- If ParseDLL is false, then append "-c" to command to produce shortened CSV output
		- If ParseDLL is true, verbose output produced allowing to search for specified DLL libraries referenced
	*/
	sysDrivePath := os.Getenv("SYSTEMDRIVE")
	prefetchParserPath := sysDrivePath + "\\windowsprefetch\\prefetch.py"
	winPrefetchPath := "C:\\Windows\\Prefetch\\"
	cmdName := "python"
	cmdArgs := []string{prefetchParserPath, "-d", winPrefetchPath}

	if r.Parameters.ParseDLL == false {
		cmdArgs = append(cmdArgs, "-c")
	}
	cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
	if err != nil {
		panic(err)
	}

	/* Begin parsing output of captured Prefetch directory */
	prefetchOutput := string(cmdOut)
	prefetchLines := strings.Split(prefetchOutput, "\n")

	/* Output is different and so requires different parsing algorithm */
	if r.Parameters.ParseDLL == false {
		for i := 1; i < len(prefetchLines)-1; i++ {
			line := strings.Split(prefetchLines[i], ",")
			// lastExe := strings.Fields(line[0])

			pr.DateExecuted = line[0] //lastExe[0] + " " + lastExe[1]
			pr.ExeName = line[3]
			pr.RunCount = line[4]

			allpr = append(allpr, pr)
			stats.NumPrefetch++
		}
	} else {

		// for each entry in line, determine which object field to populate
		// Some entries in output can run multiple lines, so must be added to appropriate array (slice)
		endOfRecord = false
		for i := 1; i < len(prefetchLines)-1; i++ {

			// if line begins with this, then split it and take Name
			if strings.HasPrefix(prefetchLines[i], "Executable Name:") {
				exeName = strings.Split(prefetchLines[i], ":")[1]
			}

			// same as above
			if strings.HasPrefix(prefetchLines[i], "Run count:") {
				runCount = strings.Split(prefetchLines[i], ":")[1]
			}

			// a bit more complicated here. If the program only has single Last Executed entry, then date/time is on same line
			// However, if there are more, then each entry is split over a new line, BELOW the 'Last Executed:' start
			// In that case, starting at j = i+1, continue and add each entry to 'lastExecuted' array until hit the end
			if strings.HasPrefix(prefetchLines[i], "Last Executed:") {
				if len(prefetchLines[i]) > 15 {

					returnDate = strings.Split(prefetchLines[i], "Executed:")[1]

				} else {

					// User can supply parameter to retrieve either earliest or latest execution date
					if r.Parameters.GetLastDate {
						timestamp := strings.Fields(prefetchLines[i+1])
						if len(timestamp) > 1 {
							returnDate = timestamp[0] + " " + timestamp[1]
						}
					} else {

						// if GetLastDate is false, get earliest date, which is last in series
						for j := i + 1; j < len(prefetchLines); j++ {
							if len(prefetchLines[j]) < 5 {
								break
							}
							// if s[j] == "" { break }

							timestamp := strings.Fields(prefetchLines[j])
							if len(timestamp) > 1 {
								execDate = timestamp[0]
								execTime = timestamp[1]

							}
						}
						returnDate = execDate + " " + execTime
					}

				}
			}

			// relatively simple as there are always 3 lines for this
			if strings.HasPrefix(prefetchLines[i], "Volume Information:") {
				volInfo.VolumeName = strings.Split(prefetchLines[i+1], ":")[1]
				volInfo.CreationDate = strings.Split(prefetchLines[i+2], ":")[1]
				volInfo.Serial = strings.Split(prefetchLines[i+3], ":")[1]
			}

			// also relatively simple as there are always lots of lines. Stop processing when the line has less then 5 chars
			if strings.HasPrefix(prefetchLines[i], "Directory Strings:") {
				for j := i + 1; j < len(prefetchLines); j++ {
					if len(prefetchLines[j]) < 5 {
						break
					}
					dirStrings = append(dirStrings, strings.Replace(prefetchLines[j], " ", "", -1))
				}
			}

			/*	Same as previous. Hoever, per program in the prefetch, this is the LAST entry, and so when done processing,
				mark the bool value "endOfRecord" as true, to enable appending prefetch object "pr" to array of prefetch records
				and resetting of temp arrays (slices)
			*/
			if strings.HasPrefix(prefetchLines[i], "Resources loaded:") {
				for j := i + 2; j < len(prefetchLines); j++ {

					if len(prefetchLines[j]) < 5 {
						endOfRecord = true
						break
					}
					resourceStr := strings.Split(prefetchLines[j], ":")[1]
					resourceStr2 := strings.Replace(resourceStr, " ", "", -1)
					resList = append(resList, resourceStr2)
				}
			}

			/*
					when we reach end of record, increment counter, assign temp arrays to Prefetch object, and append prefetch object to array
				 	After, reset temp arrays and set endOfRecord to false to begin again
			*/
			if endOfRecord {
				numPrefetch++
				var pr PrefetchRecord
				pr.ExeName = exeName
				pr.RunCount = runCount
				pr.DateExecuted = returnDate
				pr.Volume = volInfo
				pr.DirectoryStrings = dirStrings
				pr.ResourcesLoaded = resList
				allpr = append(allpr, pr)
				stats.NumPrefetch++

				/* Reinitialise variables for next prefetch entry */
				dirStrings = dirStrings[:0]
				resList = resList[:0]
				endOfRecord = false
			}
		}
	}

	/*
		Search for Exe's and DLL libraries specified in input parameters
		- if found, append entry to results slice
	*/
	for _, targetExe := range r.Parameters.SearchExe {

		if r.Parameters.Debug {
			fmt.Println("Searching for EXE: ", targetExe)
		}
		for i := 0; i < len(allpr); i++ {
			var result PrefetchResult
			if strings.Contains(allpr[i].ExeName, targetExe) {
				result.ExeName = allpr[i].ExeName
				result.ExecDate = allpr[i].DateExecuted
				result.RunCount = allpr[i].RunCount

				el.prefetch = append(el.prefetch, result)

				stats.ExesFound++
				stats.TotalHits++
			}

		}
	}

	for _, targetDLL := range r.Parameters.SearchDLL {
		if r.Parameters.Debug {
			fmt.Println("Searching for DLL:", targetDLL)
		}
		for i := 0; i < len(allpr); i++ {
			var result PrefetchResult
			for j := 0; j < len(allpr[i].ResourcesLoaded); j++ {
				if strings.Contains(allpr[i].ResourcesLoaded[j], targetDLL) {
					result.DLLName = targetDLL
					result.ExeName = allpr[i].ExeName
					result.ExecDate = allpr[i].DateExecuted
					result.RunCount = allpr[i].RunCount
					el.prefetch = append(el.prefetch, result)
					fmt.Sprintf("Exe: %s, Date: %s, RunCount: %s", result.ExeName, result.ExecDate, result.RunCount)
					stats.DLLsFound++
					stats.TotalHits++
				}
			}
		}
	}

	// marshal the results into a json string
	*out = r.buildResults(el, stats)
	timeEnd := time.Now()
	stats.Exectime = timeEnd.Sub(timeStart)
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

	prints = append(prints, fmt.Sprintf("\n-----------------\n     Prefetch Results           \n------------------"))
	// if true, print results by DLL searched, else print exe and execution date
	for _, prefetch := range el.prefetch {
		if r.Parameters.ParseDLL == true {
			prints = append(prints, fmt.Sprintf("DLL Found: %s, Executable: %s, First Run: %v, Run Count: %s", prefetch.DLLName, prefetch.ExeName,
				prefetch.ExecDate, prefetch.RunCount))
		} else {
			prints = append(prints, fmt.Sprintf("Executable Found: %s, First Run: %v, Run Count: %s", prefetch.ExeName, prefetch.ExecDate, prefetch.RunCount))
		}
	}

	for _, e := range result.Errors {
		prints = append(prints, fmt.Sprintf("error: %v", e))
	}
	err = result.GetStatistics(&stats)
	if err != nil {
		panic(err)
	}

	// prints = append(prints, fmt.Sprintf("\nStats:\n-----------------------"))
	prints = append(prints, fmt.Sprintf("Total Hits: %d", stats.TotalHits))
	// prints = append(prints, fmt.Sprintf("Prefetch Records: %d", stats.NumPrefetch))
	// prints = append(prints, fmt.Sprintf("Exe's Found: %d", stats.ExesFound))
	// prints = append(prints, fmt.Sprintf("DLL's Found: %d", stats.DLLsFound))
	// prints = append(prints, fmt.Sprintf("Execution Time: %v", stats.Exectime))
	return
}
