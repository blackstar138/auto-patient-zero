/*
	@TODO: GPL License 2016
*/

/*
	@TODO: Description of Registry module: 	i) Purpose
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
package registry /* import "mig.ninja/mig/modules/registry" */

import (
	"encoding/json"
	"fmt"
	"io"
	"mig.ninja/mig/modules"
	"os"
	"os/exec"
	// "regexp"
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
	modules.Register("registry", new(module))
}

type run struct {
	Parameters params
	Results    modules.Result
}

/* a simple parameters structure, the format is arbitrary */
type params struct {
	Rekall RekallParams `json:"rekall,omitempty"`
	RegRip RegRipParams `json:"regrip,omitempty"`
	Search SearchParams `json:"search,omitempty"`
	Debug  bool         `json:"debug, omitempty"`
}

type elements struct {
	Results []RegRecord `json:"registryresults,omitempty"`
}

type RegRecord struct {
	Hive      string   `json:"hive,omitempty"`
	Key       string   `json:"key,omitempty"`
	Value     []string `json:"value,omitempty"`
	Data      []string `json:"data,omitempty"`
	LastWrite string   `json:"lastwrite,omitempty"`
	// LastWrite time.Time `json:"lastwrite,omitempty"`
}

type RekallParams struct {
	Plugin        string   `json:"plugin,omitempty"`
	PluginOptions []string `json:"pluginoptions,omitempty"`
	CheckValues   bool     `json:"checkvalues,omitempty"`
	DumpDirectory string   `json:"dumpdirectory,omitempty"`
	TargetHives   []string `json:"targethives,omitempty"`
}

type RegRipParams struct {
	RegDirectory    string   `json:"regdirectory,omitempty"`
	ReportDirectory string   `json:"reportdirectory,omitempty"`
	Plugins         []string `json:"plugins,omitempty"`
}

type SearchParams struct {
	SearchKeys     []string  `json:"searchkeys,omitempty"`
	SearchValues   []string  `json:"searchvalues,omitempty"`
	SearchData     []string  `json:"searchdata,omitempty"`
	StartDate      time.Time `json:"startdate,omitempty"`
	EndDate        time.Time `json:"enddate,omitempty"`
	CheckDateRange bool      `json:"checkdaterange,omitempty"`
}

/* Statistic counters:
-
- Totalhits is the total number of checklist hits
- Exectim is the total runtime of all the searches
*/
type statistics struct {
	NumKeysFound      int           `json:"numkeysfound"`
	KeysSearched      int           `json:"keyssearched"`
	NumValuesFound    int           `json:"numvaluesfound"`
	NumDataFound      int           `json:"numdatafound"`
	NumHivesProc      int           `json:"numhivesproc"`
	RegRipCatExec     int           `json:"regripcatexec"`
	AutoRunsFound     int           `json:"autorunsfound"`
	ExecKeysFound     int           `json:"execkeysfound"`
	StorageKeysFound  int           `json:"storagekeysfound"`
	NetworkKeysFound  int           `json:"networkkeysfound"`
	SoftwareKeysFound int           `json:"softwarekeysfound"`
	SecurityKeysFound int           `json:"securitykeysfound"`
	SystemKeysFound   int           `json:"systemkeysfound"`
	SAMKeysFound      int           `json:"samkeysfound"`
	DefaultKeysFound  int           `json:"defaultkeysfound"`
	UsersKeysFound    int           `json:"userskeysfound"`
	WebKeysFound      int           `json:"webkeysfound"`
	TotalHits         int           `json:"totalhits"`
	Exectime          time.Duration `json:"exectime"`
}

/*
	ValidateParameters *must* be implemented by a module. It provides a method to verify that the parameters
	passed to the module conform the expected format. It must return an error if the parameters do not validate.
*/
func (r *run) ValidateParameters() (err error) {
	// if r.Parameters.CheckHosts || r.Parameters.CheckDns {
	if r.Parameters.Rekall.CheckValues {
		if r.Parameters.Rekall.DumpDirectory == "" {
			return fmt.Errorf("ValidateParameters: SearchHosts parameter is not a valid FQDN.")
		}

		if r.Parameters.RegRip.ReportDirectory == "" {
			return fmt.Errorf("ValidateParameters: If CheckValues set then must supply output directory for RegRipper reports.")
		}

		if r.Parameters.Search.SearchValues == nil && r.Parameters.Search.SearchData == nil {
			return fmt.Errorf("ValidateParameters: If CheckValues set then must supply both SearchValues & SearchData.")
		}
	}

	// if r.Parameters.Search.CheckDateRange {
	// 	if r.Parameters.Search.StartDate == nil || r.Parameters.Search.EndDate == nil {
	// 		return fmt.Errorf("ValidateParameters: If CheckDateRange set, both START and END dates must be provided.")
	// 	}
	// }

	if r.Parameters.Search.CheckDateRange {
		if r.Parameters.Search.EndDate.Before(r.Parameters.Search.StartDate) {
			return fmt.Errorf("ValidateParameters: EndDate is *BEFORE* StartDate.")
		}
	}

	// if len(r.Parameters.Search.SearchKeys) > 0 {
	// 	keys := regexp.MustCompilePOSIX(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`)
	// 	for _, key := range r.Parameters.Search.SearchKeys {
	// 		if !keys.MatchString(key) {
	// 			return fmt.Errorf("ValidateParameters: SearcKheys parameter is not a valid FQDN.")
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
	/*
		a good way to handle execution failures is to catch panics and store the panicked error into
		modules.Results.Errors, marshal that, and output the JSON string back to the caller
	*/
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

	// start a goroutine that does some work and another one that looks for an early stop signal
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
	var (
		el     elements
		stats  statistics
		Reg    RegRecord
		Allreg []RegRecord
	)
	RegMap := make(map[string]string)

	t0 := time.Now()

	sysDrivePath := os.Getenv("SYSTEMDRIVE")
	rekall := sysDrivePath + "\\Rekall\\rekal.exe"
	usrDir := os.Getenv("USERPROFILE")
	stats.TotalHits = 0 // counter for found entries

	if r.Parameters.Debug {
		// fmt.Println("Rekall at: ", rekall)
		// regRip := sysDrivePath+"\\auto_rip\\auto_rip64.exe"
		// fmt.Println("OS: ", runtime.GOOS)
	}

	if runtime.GOOS != "windows" {
		return fmt.Errorf("Error: Registry Module must run on Windows Environment only.") // Registry Module only for Windows OS machines
	}

	var HiveList []string
	if r.Parameters.Rekall.TargetHives != nil {
		HiveList = r.Parameters.Rekall.TargetHives
	} else {
		HiveList = []string{"SECURITY", "SOFTWARE", "SYSTEM", "SAM", "DEFAULT", "ntuser.dat", "Amcache.hve"}
	}

	if r.Parameters.Debug {
		fmt.Println("HiveList: ", HiveList)
	}

	/*
		- Use Rekall to dump hive list with memory offsets
		---> For each hive argument / hive in default list:
			=> Dump hive keys & last write date
			=> Scan output for target keys (with/without regex) and return result []RegRecord list
		---> If parameterised to check reg key value/data fields, then need to dump entire registry
			 and use RegRipper to parse data before scanning for entries --> Alot more work!
	*/
	if r.Parameters.Rekall.CheckValues == false {

		if r.Parameters.Debug {
			// fmt.Println("CheckValues: False --> Start extracting hives...")
			// fmt.Println("Looking for: ", r.Parameters.Search.SearchKeys)
		}
		cmdName := rekall
		cmdArgs := []string{
			"--live", "--plugin", r.Parameters.Rekall.Plugin,
		}

		cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
		if err != nil {
			panic(err)
		}

		/* Begin parsing captured output from Rekall Reg Hive dump */
		var offset, hive string
		hivesOutput := string(cmdOut)
		s := strings.Split(hivesOutput, "\n")
		if r.Parameters.Debug {
			fmt.Println("Checking hives")
		}
		for i := 2; i < len(s)-1; i++ {

			line := strings.Fields(s[i])
			offset = line[3]
			hive = line[1]

			hiveStr := strings.Split(hive, "\\")
			hiveStuff := hiveStr[len(hiveStr)-1]

			// Build Map of Hives + Memory offset to use for dumping specific hives later
			for _, curHive := range HiveList {
				_, present := RegMap[curHive]
				if hiveStuff == curHive && present == false {
					if r.Parameters.Debug {
						fmt.Println("Processing: ", curHive)
					}

					/*
						Rekall dumps multple NTUSER.DAT hives from different directories
						We look for only the NTUSER.DAT from the user directory ??
					*/
					if hive == usrDir+"ntuser.dat" && curHive == "NTUSER.DAT" {
						RegMap[curHive] = offset
					} else {
						if r.Parameters.Debug {
							fmt.Println("Setting", curHive, "with offset:", offset)
						}
						RegMap[curHive] = offset
					}
				}
			}

		}

		for hive, offset := range RegMap {
			if r.Parameters.Debug {
				fmt.Println("Processing ", hive, "....")
			}

			cmdArgs = []string{
				"--live",
				"--plugin",
				"hivedump",
				"--hive-offset",
				offset,
			}
			cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
			if err != nil {
				panic(err)
			}

			hivesOutput = string(cmdOut)
			s = strings.Split(hivesOutput, "\n")
			for i := 5; i < len(s); i++ {
				line := strings.Fields(s[i])
				if len(line) > 1 {
					stats.KeysSearched++
					switch hive {
					case "SYSTEM":
						stats.SystemKeysFound++
					case "SOFTWARE":
						stats.SoftwareKeysFound++
					case "SAM":
						stats.SAMKeysFound++
					case "ntuser.dat":
						stats.UsersKeysFound++
					case "DEFAULT":
						stats.DefaultKeysFound++
					}
				}

				/*
					Search for Exe's and DLL libraries specified in input parameters
					- if found, append entry to results slice
				*/

				for _, targetKey := range r.Parameters.Search.SearchKeys {
					if len(line) > 1 {

						/* MATCH FOUND */
						if strings.Contains(line[2], targetKey) {
							// Form LastWrite value from date + time stamp
							lastWrite1 := line[0] + " " + line[1]
							// Reg.LastWrite, _ = time.Parse(time.RFC3339, lastWrite1)
							// lastWrite, _ := time.Parse(time.RFC3339, lastWrite1)

							if r.Parameters.Search.CheckDateRange == true {
								// if lastWrite.After(r.Parameters.Search.StartDate) {

								// }
							} else {
								Reg.Hive = hive
								Reg.Key = line[2]

								Reg.LastWrite = lastWrite1
								// Reg.LastWrite, _ = time.Parse(time.RFC3339, lastWrite1)
								Allreg = append(Allreg, Reg)

								stats.NumKeysFound++
								stats.TotalHits++
							}
						}
					}
				} // End loop through target search keys

			} // End loop through RegHive Dump

		} // End looping through hiveMap

	} else { // case: Rekall.CheckValues == true

		// cmdName := rekall
		// cmdArgs := []string{
		// 	"--live", "--plugin", "regdump", "--dump_dir", r.Parameters.Rekall.dumpdirectory,
		// }

		// cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
		// if err != nil { panic(err)	}

		// /* Begin parsing captured output from Rekall Reg Hive dump */
		// var offset, hive string
		// regDumpOutput := string(cmdOut)
		// s := strings.Split(regDumpOutput, "\n")
		// for i := 2; i < len(s)-1; i++ {

		// 	line := strings.Fields(s[i])
		// 	offset = line[3]
		// 	hive = line[1]

		// 	hiveStr := strings.Split(hive, "\\")
		// 	hiveStuff := hiveStr[len(hiveStr)-1]

		// 	// Build Map of Hives + Memory offset to use for dumping specific hives later
		// 	for _, curHive := range HiveList {
		// 		_, present := RegMap[curHive]
		// 		if hiveStuff == curHive && present == false {

		// 			// Rekall dumps multple NTUSER.DAT hives from different directories
		// 			// We look for only the NTUSER.DAT from the user directory ??
		// 			if hive == usrDir+"ntuser.dat" && curHive == "NTUSER.DAT" {
		// 				RegMap[curHive] = offset
		// 			} else {
		// 				RegMap[curHive] = offset
		// 			}
		// 		}
		// 	}

		// }
	}

	/*
		   ------------------------------------------------------
			After performing all Registry searches, build results
		   ------------------------------------------------------
	*/
	el.Results = Allreg
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
	prints = append(prints, fmt.Sprintf("\n-----------------\n     Registry Results           \n------------------"))
	for _, reg := range el.Results {
		prints = append(prints, fmt.Sprintf("Hive: %s, Reg Key Found: %s, Last Modified: %v", reg.Hive, reg.Key, reg.LastWrite))
		prints = append(prints, fmt.Sprintf("Hive: %s, Reg Key Found: %s, Last Modified: %v", reg.Hive, reg.Key, reg.LastWrite))
	}

	for _, e := range result.Errors {
		prints = append(prints, fmt.Sprintf("error: %v", e))
	}
	err = result.GetStatistics(&stats)
	if err != nil {
		panic(err)
	}

	// prints = append(prints, fmt.Sprintf("Keys Searched  : %d", stats.KeysSearched))
	// prints = append(prints, fmt.Sprintf("Keys Found     : %d", stats.NumKeysFound))
	// prints = append(prints, fmt.Sprintf("Values Found   : %d", stats.NumValuesFound))
	// prints = append(prints, fmt.Sprintf("Hives Processed: %d", stats.NumHivesProc))
	prints = append(prints, fmt.Sprintf("Total Hits     : %d", stats.TotalHits))
	// prints = append(prints, fmt.Sprintf("Exec Time      : %v", stats.Exectime))

	return
}
