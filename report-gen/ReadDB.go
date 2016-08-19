package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	// "reflect"
	// "io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

/* File Module Structs */
type SearchResults map[string]searchresult
type RegRecords map[string][]RegRecord
type PrefetchRecs map[string][]PrefetchResult

type searchresult []matchedfile

type matchedfile struct {
	File     string   `json:"file"`
	Search   search   `json:"search"`
	FileInfo fileinfo `json:"fileinfo"`
}

type fileinfo struct {
	Size   float64 `json:"size"`
	Mode   string  `json:"mode"`
	Mtime  string  `json:"lastmodified"`
	SHA256 string  `json:"sha256,omitempty"`
}

type Parameters struct {
	Searches map[string]search `json:"searches,omitempty"`
}

type search struct {
	Description  string   `json:"description,omitempty"`
	Paths        []string `json:"paths"`
	Contents     []string `json:"contents,omitempty"`
	Names        []string `json:"names,omitempty"`
	Sizes        []string `json:"sizes,omitempty"`
	Modes        []string `json:"modes,omitempty"`
	Mtimes       []string `json:"mtimes,omitempty"`
	MD5          []string `json:"md5,omitempty"`
	SHA1         []string `json:"sha1,omitempty"`
	SHA2         []string `json:"sha2,omitempty"`
	SHA3         []string `json:"sha3,omitempty"`
	Options      options  `json:"options,omitempty"`
	checks       []check
	checkmask    checkType
	isactive     bool
	iscurrent    bool
	currentdepth uint64
}

type options struct {
	MaxDepth     float64  `json:"maxdepth"`
	MaxErrors    float64  `json:"maxerrors"`
	RemoteFS     bool     `json:"remotefs,omitempty"`
	MatchAll     bool     `json:"matchall"`
	Macroal      bool     `json:"macroal"`
	Mismatch     []string `json:"mismatch"`
	MatchLimit   float64  `json:"matchlimit"`
	Debug        string   `json:"debug,omitempty"`
	ReturnSHA256 bool     `json:"returnsha256,omitempty"`
	Decompress   bool     `json:"decompress,omitempty"`
}

type checkType uint64

const (
	checkContent checkType = 1 << (64 - 1 - iota)
	checkName
	checkSize
	checkMode
	checkMtime
	checkMD5
	checkSHA1
	checkSHA256
	checkSHA384
	checkSHA512
	checkSHA3_224
	checkSHA3_256
	checkSHA3_384
	checkSHA3_512
)

type check struct {
	code                   checkType
	matched                uint64
	matchedfiles           []string
	value                  string
	regex                  *regexp.Regexp
	minsize, maxsize       uint64
	minmtime, maxmtime     time.Time
	inversematch, mismatch bool
}

/* Prefetch Module Structs */

type params struct {
	// Prefetch Module
	ParseDLL    bool     `json:"parsedll"`
	SearchExe   []string `json:"searchexe"`
	SearchDLL   []string `json:"searchdll"`
	GetLastDate bool     `json:"getlastdate"`
	// Debug       bool     `json:"debug"`

	// Registry Module
	Rekall RekallParams `json:"rekall,omitempty"`
	RegRip RegRipParams `json:"regrip,omitempty"`
	Search SearchParams `json:"search,omitempty"`
	Debug  bool         `json:"debug, omitempty"`

	// Example Module
	GetHostname  bool     `json:"gethostname"`
	GetAddresses bool     `json:"getaddresses"`
	LookupHost   []string `json:"lookuphost"`
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

/* Registry Module Structs */
type RegRecord struct {
	Hive string `json:"hive,omitempty"`
	Key  string `json:"key,omitempty"`
	// Value     []string `json:"value,omitempty"`
	// Data      []string `json:"data,omitempty"`
	LastWrite string `json:"lastwrite,omitempty"`
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

/* Example Module Structs */
// type params struct {
// 	GetHostname  bool     `json:"gethostname"`
// 	GetAddresses bool     `json:"getaddresses"`
// 	LookupHost   []string `json:"lookuphost"`
// }

/* MIG Core Structs */
type Command struct {
	ID         int       `json:"id"`
	Action     Action    `json:"action"`
	Agent      Agent     `json:"agent"`
	Results    []Result  `json:"results"`
	StartTime  time.Time `json:"starttime"`
	FinishTime time.Time `json:"finishtime"`
	// Status can be one of:
	// sent: the command has been sent by the scheduler to the agent
	// success: the command has successfully ran on the agent and been returned to the scheduler
	// cancelled: the command has been cancelled by the investigator
	// expired: the command has been expired by the scheduler
	// failed: the command has failed on the agent and been returned to the scheduler
	// timeout: module execution has timed out, and the agent returned the command to the scheduler
	Status string `json:"status"`
}

const (
	StatusSent      string = "sent"
	StatusSuccess   string = "success"
	StatusCancelled string = "cancelled"
	StatusExpired   string = "expired"
	StatusFailed    string = "failed"
	StatusTimeout   string = "timeout"
)

// Result implement the base type for results returned by modules.
// All modules must return this type of result. The fields are:
//
// - FoundAnything: a boolean that must be set to true if the module ran
//                  a search that returned at least one positive result
//
// - Success: a boolean that must be set to true if the module ran without
//            fatal errors. soft errors are reported in Errors
//
// - Elements: an undefined type that can be customized by the module to
//             contain the detailled results
//
// - Statistics: an undefined type that can be customized by the module to
//               contain some information about how it ran
//
// - Errors: an array of strings that contain non-fatal errors encountered
//           by the module
type Result struct {
	FoundAnything bool        `json:"foundanything"`
	Success       bool        `json:"success"`
	Elements      interface{} `json:"elements"`
	Statistics    interface{} `json:"statistics"`
	Errors        []string    `json:"errors"`
}

type elements struct {
	// Exmaple Module Elements
	Hostname     string              `json:"hostname,omitempty"`
	Addresses    []string            `json:"addresses,omitempty"`
	LookedUpHost map[string][]string `json:"lookeduphost,omitempty"`

	// Prefetch Module Elements
	// prefetch []PrefetchResult `json:"prefetchresults,omitempty"`

	// Registry Module Elements
	results []RegRecord `json:"results,omitempty"`
}

type statistics struct {
	// Registry Module Statistics
	NumKeysFound      int `json:"numkeysfound"`
	KeysSearched      int `json:"keyssearched"`
	NumValuesFound    int `json:"numvaluesfound"`
	NumDataFound      int `json:"numdatafound"`
	NumHivesProc      int `json:"numhivesproc"`
	RegRipCatExec     int `json:"regripcatexec"`
	AutoRunsFound     int `json:"autorunsfound"`
	ExecKeysFound     int `json:"execkeysfound"`
	StorageKeysFound  int `json:"storagekeysfound"`
	NetworkKeysFound  int `json:"networkkeysfound"`
	SoftwareKeysFound int `json:"softwarekeysfound"`
	SecurityKeysFound int `json:"securitykeysfound"`
	SystemKeysFound   int `json:"systemkeysfound"`
	SAMKeysFound      int `json:"samkeysfound"`
	DefaultKeysFound  int `json:"defaultkeysfound"`
	UsersKeysFound    int `json:"userskeysfound"`
	WebKeysFound      int `json:"webkeysfound"`

	// Prefetch Module Statistics
	DLLsFound   int `json:"dllsfound"`
	ExesFound   int `json:"exesfound"`
	NumPrefetch int `json:"numprefetch"`

	// File Module Statistics
	Filescount float64 `json:"filescount"`
	Openfailed float64 `json:"openfailed"`
	Totalhits  float64 `json:"totalhits"`
	Exectime   string  `json:"exectime"`
}

type Action struct {
	ID             int            `json:"id"`
	Name           string         `json:"name"`
	Target         string         `json:"target"`
	Description    Description    `json:"description,omitempty"`
	Threat         Threat         `json:"threat,omitempty"`
	ValidFrom      time.Time      `json:"validfrom"`
	ExpireAfter    time.Time      `json:"expireafter"`
	Operations     []Operation    `json:"operations"`
	PGPSignatures  []string       `json:"pgpsignatures"`
	Investigators  []Investigator `json:"investigators,omitempty"`
	Status         string         `json:"status,omitempty"`
	StartTime      time.Time      `json:"starttime,omitempty"`
	FinishTime     time.Time      `json:"finishtime,omitempty"`
	LastUpdateTime time.Time      `json:"lastupdatetime,omitempty"`
	Counters       ActionCounters `json:"counters,omitempty"`
	SyntaxVersion  uint16         `json:"syntaxversion,omitempty"`
}

type Investigator struct {
	ID             float64   `json:"id,omitempty"`
	Name           string    `json:"name"`
	PGPFingerprint string    `json:"pgpfingerprint"`
	PublicKey      []byte    `json:"publickey,omitempty"`
	PrivateKey     []byte    `json:"privatekey,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdat"`
	LastModified   time.Time `json:"lastmodified"`
	IsAdmin        bool      `json:"isadmin"`
}

// Some counters used to track the completion of an action
type ActionCounters struct {
	Sent      int `json:"sent,omitempty"`
	Done      int `json:"done,omitempty"`
	InFlight  int `json:"inflight,omitempty"`
	Success   int `json:"success,omitempty"`
	Cancelled int `json:"cancelled,omitempty"`
	Expired   int `json:"expired,omitempty"`
	Failed    int `json:"failed,omitempty"`
	TimeOut   int `json:"timeout,omitempty"`
}

// a description is a simple object that contains detail about the
// action's author, and it's revision.
type Description struct {
	Author   string  `json:"author,omitempty"`
	Email    string  `json:"email,omitempty"`
	URL      string  `json:"url,omitempty"`
	Revision float64 `json:"revision,omitempty"`
}

// a threat provides the investigator with an idea of how dangerous
// a the compromission might be, if the indicators return positive
type Threat struct {
	Ref    string `json:"ref,omitempty"`
	Level  string `json:"level,omitempty"`
	Family string `json:"family,omitempty"`
	Type   string `json:"type,omitempty"`
}

// an operation is an object that maps to an agent module.
// the parameters of the operation are passed to the module as an argument,
// and thus their format depends on the module itself.
type Operation struct {
	Module     string      `json:"module"`
	Parameters interface{} `json:"parameters"`

	// If WantCompressed is set in the operation, the parameters
	// will be compressed in PostAction() when the client sends the
	// action to the API. This will also result in IsCompressed being
	// marked as true, so the receiving agent knows it must decompress
	// the parameter data.
	IsCompressed   bool `json:"is_compressed,omitempty"`
	WantCompressed bool `json:"want_compressed,omitempty"`
}

type Agent struct {
	ID      int
	Name    string
	Version string
}

type Record struct {
	ActionID  int
	CommandID int
	Module    string
	Search    string
	Agent     string
	Status    string
	// Artefacts map[string]time.Time
	Artefacts map[string]string
}

type TimeEntry struct {
	ActionID int
	Agent    string
	Module   string
	Status   string
	Time     string
}

type Weight struct {
	Name  string
	Score int
}

func main() {

	welcome := `     


--------------------------------------------------------------------------------------------------
--------------------------------------------------------------------------------------------------


			            .-._   _ _ _ _ _ _ _ _
			 .-''-.__.-'Oo  '-' ' ' ' ' ' ' ' '-.
			'.___ '    .   .--_'-' '-' '-' _'-' '._
			 V: V 'vv-'   '_   '.       .'  _..' '.'.
			   '=.____.=_.--'   :_.__.__:_   '.   : :
			           (((____.-'        '-.  /   : :
			                             (((-'\ .' /
			                           _____..'  .'
			                          '-._____.-'

			       !!! Welcome to Mozilla MIG ReadDB !!!
	
--------------------------------------------------------------------------------------------------
--------------------------------------------------------------------------------------------------
	`

	fmt.Println(welcome)

	debugOn := false
	var records []Record
	var destFile string

	/* Query user for modules to process */
	modules := strings.Fields(queryUser("[Config] Which modules should we parse? (Blank for default)"))

	if len(modules) == 0 {
		modules = []string{"file", "registry", "prefetch"}
		fmt.Println("[config] Processing Modules: [file, registry, prefetch]")
	}

	/* Query user for specific MIG action in PostgreSQL DB to search for */
	searchAction := queryUser("[Search] Which action to search for? (Blank for all)")
	var (
		ActionID int
		err      error
	)
	if searchAction != "" {
		ActionID, err = strconv.Atoi(searchAction)
		if err != nil {
			// handle error
			fmt.Println(err)
			ActionID = 0
		}
	} else {
		ActionID = 0
	}

	if ActionID == 0 {
		fmt.Println("[info] Processing all actions in MIG Database...")
	}

	/* Query user on output mode (stdout/csv/both) -- defualts to both */
	outputMode := queryUser("[config] Choose output mode [stdout/csv/file/timeline] (Defaults to stdout)")
	switch outputMode {
	case "stdout":
		fmt.Println("[info] Output to stdout...")
		// fmt.Println("[info] Output to default stdout...")

	case "csv":
		fmt.Println("[info] Output to csv file...")
		destFile = queryUser("[config] Choose output file")

	case "file":
		fmt.Println("[info] Output to in full to file...")
		destFile = queryUser("[config] Choose output file")

	case "timeline":
		fmt.Println("[info] Output to in html timeline...")

	default:
		outputMode = "stdout"
		fmt.Println("[info] Output to default stdout...")
	}

	/* Query user whether to show debug statements */
	usrDebug := queryUser("[Config] Do you want debug on?")
	if usrDebug == "y" {
		debugOn = true
	}

	divide := `
--------------------------------------------------------------------------------------------------
--------------------------------------------------------------------------------------------------`

	fmt.Println(divide)
	// Q) Do you want debug on? y/n
	fmt.Println("\nQuerying...\n")
	time.Sleep(2 * time.Second)

	commands, errors := queryDB()
	if errors != nil {
		for i := 0; i < len(errors); i++ {
			fmt.Println(errors[i])
		}
		return
	}

	// var ArtefactTimes map[string][]TimeEntry

	// Add ArtefactTimes to return argument of processResults()
	prints, records, artefactTimes := processResults(commands, modules, ActionID)

	if debugOn {
		head := "Action, Command, Cmd Status, Threat Lvl, Threat Type, Found Anything, Success, Module, Search, Agent, Artefact, Last Modified, Size, SHA256"
		fmt.Println(head)
		for j := 0; j < len(prints); j++ {
			fmt.Println(prints[j])
		}
	}

	// Add ArtefactTimes to input argument for printResults()
	printResults(ActionID, records, artefactTimes, modules, outputMode, destFile)

}

func queryUser(question string) (answer string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s > ", question)
	text, _ := reader.ReadString('\n')
	text = strings.Replace(text, "\n", "", -1)

	return strings.ToLower(text)
}

func queryDB() (commands []Command, errors []string) {
	fmt.Println("[info] Entering queryDB")

	user := "migadmin" //"migapi"
	pass := "NK8z8Y4XP2Pfkc1-VKRZ83ZwVjB2CY8W"
	host := "127.0.0.1"
	port := 5432
	dbname := "mig"
	sslmode := "disable"

	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user, pass, host, port, dbname, sslmode)
	db, err := sql.Open("postgres", url)
	err = db.Ping()
	if err != nil {
		log.Fatal("Error: Could not establish a connection with the database")
	}

	rows, err := db.Query(`SELECT commands.id, commands.status, commands.results, commands.starttime, commands.finishtime,
			actions.id, actions.name, actions.target, actions.description, actions.threat,
			actions.operations, actions.validfrom, actions.expireafter,
			actions.pgpsignatures, actions.syntaxversion,
			agents.id, agents.name, agents.version
			FROM commands, actions, agents
			WHERE commands.actionid=actions.id AND commands.agentid=agents.id AND commands.status='success'`) // AND actions.id=$1`, actionid)

	if rows != nil {
		defer rows.Close()
	} else {
		errors = append(errors, "rows are empty")
	}
	if err != nil {
		errors = append(errors, fmt.Sprintf("Error while finding commands: '%v'", err))
	}

	for rows.Next() {
		var jRes, jDesc, jThreat, jOps, jSig []byte

		cmd := new(Command)
		err = rows.Scan(&cmd.ID, &cmd.Status, &jRes, &cmd.StartTime, &cmd.FinishTime,
			&cmd.Action.ID, &cmd.Action.Name, &cmd.Action.Target, &jDesc, &jThreat, &jOps,
			&cmd.Action.ValidFrom, &cmd.Action.ExpireAfter, &jSig, &cmd.Action.SyntaxVersion,
			&cmd.Agent.ID, &cmd.Agent.Name, &cmd.Agent.Version)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to retrieve command: '%v'", err))
		}

		err = json.Unmarshal(jRes, &cmd.Results)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to unmarshal command results: '%v'", err))
		}

		err = json.Unmarshal(jDesc, &cmd.Action.Description)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to unmarshal action description: '%v'", err))
		}
		err = json.Unmarshal(jThreat, &cmd.Action.Threat)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to unmarshal action threat: '%v'", err))
		}
		err = json.Unmarshal(jOps, &cmd.Action.Operations)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to unmarshal action operations: '%v'", err))
		}
		err = json.Unmarshal(jSig, &cmd.Action.PGPSignatures)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to unmarshal action signatures: '%v'", err))
		}

		commands = append(commands, *cmd)
	}

	if err := rows.Err(); err != nil {
		errors = append(errors, fmt.Sprintf("Failed to complete database query: '%v'", err))
	}

	fmt.Println("[info] Leaving queryDB")

	// return rows, errors
	return commands, errors
}

func processResults(commands []Command, modules []string, targetAction int) (prints []string, records []Record, ArtefactTimes map[string][]TimeEntry) {

	fmt.Println("[info] Entering processResults")

	var (
		rec Record
		tr  TimeEntry
		// altr          []TimeEntry
		checkCommand bool
		// ArtefactTimes map[string][]TimeEntry
	)

	ArtefactTimes = make(map[string][]TimeEntry)

	checkCommand = false

	for _, cmd := range commands {

		if targetAction != 0 {
			checkCommand = true
		}
		if checkCommand && cmd.Action.ID != targetAction {

			continue
		} else {

			numOps := len(cmd.Action.Operations)
			for i := 0; i < numOps; i++ {

				/*
					@CURRENT: Parsing File module search results & statistics
				*/
				for _, module := range modules {
					if cmd.Action.Operations[i].Module == module {
						rec.ActionID = cmd.Action.ID
						rec.Agent = cmd.Agent.Name
						rec.CommandID = cmd.ID
						rec.Module = cmd.Action.Operations[i].Module
						rec.Status = cmd.Status

						// tr.Agent = cmd.Action.Name
						// tr.ActionID = cmd.Action.ID
						// tr.Module = module
						// tr.Status = cmd.Status

						// arts := make(map[string]time.Time)
						arts := make(map[string]string)

						base := fmt.Sprintf("%d, %d, %s, %s, %s, %t, %t, %s", cmd.Action.ID, cmd.ID, cmd.Status, cmd.Action.Threat.Level,
							cmd.Action.Threat.Type, cmd.Results[i].FoundAnything, cmd.Results[i].Success, cmd.Action.Operations[i].Module)

						switch rec.Module {
						case "file":

							var el SearchResults
							buff, err := json.Marshal(cmd.Results[i].Elements)
							if err != nil {
								panic(err)
							}

							err = json.Unmarshal(buff, &el)

							for label, sr := range el {
								rec.Search = label

								for _, mf := range sr {
									out := fmt.Sprintf("%s, %s, %s, %s, %s, %.0f", base, label, cmd.Agent.Name, mf.File, mf.FileInfo.Mtime, mf.FileInfo.Size)

									// time.RFC3339
									// p.Before, err = time.Parse(time.RFC3339, value)
									// a.ValidFrom, err = time.Parse("2014-01-01T00:00:00.0Z", orders[1])
									// layout := "2006-01-02T15:04:05.000Z"
									// layonut := "2014-01-01T00:00:00.0Z"
									// t, err := time.Parse(layout, str)
									// fmt.Println(mf.FileInfo.Mtime)
									// arts[mf.File], err = time.Parse(layout, mf.FileInfo.Mtime)
									// 	// arts[key], err = time.Parse(layout, exeDate)
									arts[mf.File] = mf.FileInfo.Mtime
									tr.Time = mf.FileInfo.Mtime

									strings.ToLower(mf.FileInfo.SHA256)
									if mf.FileInfo.SHA256 != "" {
										out += fmt.Sprintf(",%s", strings.ToLower(mf.FileInfo.SHA256))
									}

									prints = append(prints, out)

								}

								rec.Artefacts = arts
							}

						case "registry":
							// var el []RegRecord
							var el RegRecords
							buff, err := json.Marshal(cmd.Results[i].Elements)
							if err != nil {
								panic(err)
							}

							err = json.Unmarshal(buff, &el)

							// regrecords = el.registryresults
							// fmt.Println("Reg Records List: ", el)
							countReg := 0
							// for _, reg := range el.registryresults {
							for _, record := range el {
								for _, reg := range record {
									countReg++
									// fmt.Println(countReg, " - reg results")
									arts[reg.Key] = reg.LastWrite
									out := fmt.Sprintf("%s, %s, %s, %s, %v", base, cmd.Agent.Name, reg.Hive, reg.Key, reg.LastWrite)
									prints = append(prints, out)
								}

							}

							rec.Artefacts = arts

						case "prefetch":
							var el PrefetchRecs
							buff, err := json.Marshal(cmd.Results[i].Elements)
							if err != nil {
								panic(err)
							}

							err = json.Unmarshal(buff, &el)
							for _, prefRec := range el {
								for _, pref := range prefRec {
									arts[pref.ExeName] = pref.ExecDate
									out := fmt.Sprintf("%s, %s, %s, %s, %s", base, cmd.Agent.Name, pref.ExeName, pref.ExecDate, pref.RunCount)
									prints = append(prints, out)
								}
							}

							rec.Artefacts = arts

						}
						records = append(records, rec)
					}
				} // end of loop through modules

			} // end of loop through Action operations
		} // end of CheckCommand if/else stmt

	} // end of loop through commands

	// Now we try to build map[artefact][]TimeEntry using the information we already have
	// fmt.Println("\n\n------------------------------------------\n Testing Bulding map[Artefact][]TimeRecord\n-----------------------------------------------")
	Times := make([]TimeEntry, 0)
	// var Times []TimeEntry
	for _, curRec := range records {
		found := false
		tmpTimes := make([]TimeEntry, 0)
		// ArtefactTimes["art"] = make([]TimeEntry, 0)
		// fmt.Println(ArtefactTimes)
		for k, v := range curRec.Artefacts {
			if k != "" {
				// fmt.Println("Checking artefact:", k)
				tr.Agent = curRec.Agent
				tr.Module = curRec.Module
				tr.Status = curRec.Status
				tr.ActionID = curRec.ActionID
				if len(v) > 19 {
					v = v[0:19]
				}
				tr.Time = v

				k = strings.Replace(k, "\\", "/", -1)

				if len(ArtefactTimes[k]) == 0 {
					ArtefactTimes[k] = make([]TimeEntry, 0)
				}
				Times = ArtefactTimes[k]
				// fmt.Println("Times:", Times)
				if len(Times) == 0 {
					Times = append(Times, tr)
					ArtefactTimes[k] = Times
				} else {
					for _, entry := range Times {
						if entry.Agent == tr.Agent {
							found = true
						}

						if found == false {
							tmpTimes = append(Times, tr)
							ArtefactTimes[k] = tmpTimes
						}
					}
				}
				// if _, present := ArtefactTimes[k]; present == false {
				// 	tr.Time = v
				// }
			}
		}
	}

	// for k, v := range ArtefactTimes {
	// 	fmt.Println("Art:", k)
	// 	fmt.Println("TimeEntry:", v, "\n")
	// }
	// fmt.Println(ArtefactTimes)
	// fmt.Println("------------------------------------------\n Testing Printing map[Artefact][]TimeRecord\n-----------------------------------------------")

	fmt.Println("[info] Leaving processResults, with", len(records), "records")
	return prints, records, ArtefactTimes
}

func printResults(actionID int, records []Record, ArtefactTimes map[string][]TimeEntry, modules []string, outputMode string, destFile string) {
	fmt.Println("[info] Processing Collected Records...")

	fmt.Println("[info] There are", len(records), "records")

	var prints []string
	var filename string

	switch outputMode {

	/* @TODO: Order timeline by Artefact -> show machines on timeline!!!
	- Run new 'hunt' on multiple machines with file/prefetch/registry
	- add CSS style/colours to output?
	- Need to distinguish between HTML paragraphs better - formatting??
	*/
	case "timeline":
		fmt.Println("[info] preparing timeline html output")
		HtmlHeader := `
<!DOCTYPE HTML>
<style type="text/css" media="screen">
/* Style the list */
ul.tab {
	list-style-type: none;
	margin: 0;
	padding: 0;
	overflow: hidden;
	border: 1px solid #ccc;
	background-color: #f1f1f1;
}
/* Float the list items side by side */
ul.tab li {
	float: left;
}
/* Style the links inside the list items */
ul.tab li a {
	display: inline-block;
	color: black;
	text-align: center;
	padding: 14px 16px;
	text-decoration: none;
	transition: 0.3s;
	font-size: 17px;
}
/* Change background color of links on hover */
ul.tab li a:hover {
	background-color: #ddd;
}
/* Create an active/current tablink class */
ul.tab li a:focus, .active {
	background-color: #ccc;
}
/* Style the tab content */
.tabcontent {
	display: none;
	padding: 6px 12px;
	border: 1px solid #ccc;
	border-top: none;
}
</style>
<script>
	function openEvent(evt, eventName) {
		var i, tabcontent, tablinks;
		tabcontent = document.getElementsByClassName("tabcontent");
		for (i = 0; i < tabcontent.length; i++) {
			tabcontent[i].style.display = "none";
		}
		tablinks = document.getElementsByClassName("tablinks");
		for (i = 0; i < tablinks.length; i++) {
			tablinks[i].className = tablinks[i].className.replace(" active", "");
		}
		document.getElementById(eventName).style.display = "block";
		evt.currentTarget.className += " active";
	}
</script>
<html>
	<head>
		<title>Threat Search | Malware Family | Timeline </title>
		<style type="text/css">
		body, html {
		font-family: sans-serif;
		}
		</style>
		<script src="../../dist/vis.js"></script>
		<link href="../../dist/vis.css" rel="stylesheet" type="text/css" />
		<script src="../googleAnalytics.js"></script>
	</head>
	<body>
		<ul class="tab">
			<li><a href="#" class="tablinks" onclick="openEvent(event, 'Hosts')">Hosts</a></li>
			<li><a href="#" class="tablinks" onclick="openEvent(event, 'Artefacts')">Artefacts</a></li>
		</ul>
			`
		HtmlFooter := `
	</body>
</html>
			`

		prints = append(prints, HtmlHeader)
		prints = append(prints, `		<div id="Hosts" class="tabcontent">`)
		for _, module := range modules {
			for m := 0; m < len(records); m++ {
				if records[m].Module == module {

					paragraph := fmt.Sprintf("		<p>%s  - %s Events</p>", records[m].Agent, records[m].Module)
					prints = append(prints, paragraph)

					div := fmt.Sprintf(`		<div id="%s-%s"></div>`, records[m].Agent, module)
					prints = append(prints, div)

					script := `		<script type="text/javascript">`
					prints = append(prints, script)

					// var container = document.getElementById('%s-%s');
					container := fmt.Sprintf("			var container = document.getElementById('%s-%s');", records[m].Agent, module)
					prints = append(prints, container, "		var items = new vis.DataSet([")

					countArts := 0
					for k, v := range records[m].Artefacts {
						if k != "" {
							if len(v) > 19 {
								v = v[0:19]
							}
							countArts++
							item := fmt.Sprintf("				{id: %d, content: '%s', start: '%s', title: '%s Key'},", countArts, k, v, module) //[4:len(value)]
							prints = append(prints, item)
						}
					}

					prints = append(prints, "			]);")
					prints = append(prints, "			var options = {};")
					prints = append(prints, "			var timeline = new vis.Timeline(container, items, options);")
					prints = append(prints, "		</script>")

				}
			}

		}
		prints = append(prints, `		</div>`)

		// prints2 = append(prints2, HtmlHeader)
		prints = append(prints, `		<div id="Artefacts" class="tabcontent">`)
		for art, tr := range ArtefactTimes {
			// for art, Tr := range ArtefactTimes {
			paragraph := fmt.Sprintf("	<p>Artefact - %s</p>", art)
			prints = append(prints, paragraph)

			div := fmt.Sprintf(`	<div id="%s"></div>`, art)
			prints = append(prints, div)

			script := `	<script type="text/javascript">`
			prints = append(prints, script)

			// var container = document.getElementById('%s-%s');
			container := fmt.Sprintf("		var container = document.getElementById('%s');", art)
			prints = append(prints, container, "		var items = new vis.DataSet([")

			for i := 0; i < len(tr); i++ {
				if len(tr[i].Time) > 19 {
					tr[i].Time = tr[i].Time[0:19]
				}

				if tr[i].Agent != "" {
					item := fmt.Sprintf("			{id: %d, content: '%s', start: '%s', title: '%s Key'},", i, tr[i].Agent, tr[i].Time, tr[i].Module)
					prints = append(prints, item)
				}
			}

			prints = append(prints, "		]);")
			prints = append(prints, "		var options = {};")
			prints = append(prints, "		var timeline = new vis.Timeline(container, items, options);")
			prints = append(prints, "	</script>")
		}
		prints = append(prints, `		</div>`)

		prints = append(prints, HtmlFooter)
		fmt.Println("[info]", len(prints), "lines of HTML")

	case "csv":
		for _, module := range modules {
			for m := 0; m < len(records); m++ {
				if records[m].Module == module {

					for k, v := range records[m].Artefacts {
						out := fmt.Sprintf("%d, %s, %s, %s, %s, %s", records[m].ActionID, module, records[m].Agent, records[m].Search, k, v)
						prints = append(prints, out)
					}
				}
			}
		}

		for i := 0; i < len(prints); i++ {
			fmt.Println(prints[i])
		}
		// prints = append(prints,)
	default:

		for _, module := range modules {
			prints = append(prints, fmt.Sprintf("Module: %s\n---------------------------", module))
			for m := 0; m < len(records); m++ {
				if records[m].Module == module {
					// fmt.Println("Module:", records[m].Module)
					prints = append(prints, fmt.Sprintf("Agent: %s", records[m].Agent))
					// fmt.Println("Record:", m)
					// fmt.Println("Action ID:", records[m].ActionID)
					// fmt.Println("Command ID:", records[m].CommandID)
					// fmt.Println("Status :", records[m].Status)

					prints = append(prints, fmt.Sprintf("Search: %s", records[m].Search))
					var countRecs int
					for k, v := range records[m].Artefacts {
						countRecs++
						// fmt.Printf("Artefact %d : File : %s", countRecs, k)
						prints = append(prints, fmt.Sprintf("Artefact %d : File : %s", countRecs, k))
						prints = append(prints, fmt.Sprintf("           : Date : %s", v))
						// fmt.Println("---> Modified:", v, "\n")
					}
					prints = append(prints, "\n")
				}

			}
			prints = append(prints, "--------------------------------------------------")

		}
	}

	if actionID == 0 {
		filename = "All-Actions"
	} else {
		filename = fmt.Sprintf("Action-%d", actionID)
	}

	if outputMode == "timeline" {
		f, err := os.Create("/media/sf_Transit/MIG/html/" + filename + ".html")
		if err != nil {
			panic(err)
		}
		w := bufio.NewWriter(f)
		defer f.Close()
		for i := 0; i < len(prints); i++ {
			_, err := w.WriteString(prints[i] + "\n")
			if err != nil {
				panic(err)
			}

		}

		w.Flush()
	} else {
		if outputMode == "file" {
			filename = filename + ".txt"
		} else {
			filename = filename + ".csv"
		}

		f, err := os.Create("/media/sf_Transit/MIG/output/" + filename)
		if err != nil {
			panic(err)
		}
		w := bufio.NewWriter(f)
		defer f.Close()
		for i := 0; i < len(prints); i++ {
			_, err := w.WriteString(prints[i] + "\n")
			if err != nil {
				panic(err)
			}

		}

		w.Flush()
	}

	//scp -i "blackstar138_aws.pem" ~/Transit/MIG/ReadDB* ubuntu@ec2-52-209-207-211.eu-west-1.compute.amazonaws.com:/home/ubuntu/
	cmdName := "scp"
	cmdArgs := []string{"-i", `"~/priv/blackstar138_aws.pem"`, filename, "ec2-user@ec2-52-209-91-245.eu-west-1.compute.amazonaws.com:/var/www/html/vis/examples/timeline"}

	if r.Parameters.ParseDLL == false {
		cmdArgs = append(cmdArgs, "-c")
	}
	cmdOut, err := exec.Command(cmdName, cmdArgs...).Output()
	if err != nil {
		panic(err)
	}

	// if outputMode == "file" {
	// 	filename = filename + ".txt"
	// } else {
	// 	filename = filename + ".csv"
	// }

	// f, err := os.Create("/media/sf_Transit/MIG/output/"+filename)
	// if err != nil {
	// 	panic(err)
	// }
	// w := bufio.NewWriter(f)
	// defer f.Close()
	// for i := 0; i < len(prints); i++ {
	// 	_, err := w.WriteString(prints[i] + "\n")
	// 	if err != nil {
	// 		panic(err)
	// 	}

	// }

	// w.Flush()

	// } else {

	// 	f, err := os.Create("/media/sf_Transit/MIG/html/"+filename)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	w := bufio.NewWriter(f)
	// 	defer f.Close()
	// 	for i := 0; i < len(prints); i++ {
	// 		_, err := w.WriteString(prints[i] + "\n")
	// 		if err != nil {
	// 			panic(err)
	// 		}

	// 	}

	// 	w.Flush()
	// }

}
