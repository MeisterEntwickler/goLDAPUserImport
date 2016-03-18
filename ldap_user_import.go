package main

//----- Packages -----
import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color" //-- CLI Colour
	"github.com/hornbill/goApiLib"
	"github.com/hornbill/ldap"    //-- Hornbill Clone of "github.com/mavricknz/ldap"
	"github.com/hornbill/pb"      //--Hornbil Clone of "github.com/cheggaaa/pb"
	"github.com/tcnksm/go-latest" //-- For Version checking
)

//----- Constants -----
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const version = "1.7.0"

//----- Variables -----
var ldapImportConf ldapImportConfStruct
var xmlmcInstanceConfig xmlmcConfig
var ldapUsers []*ldap.Entry
var xmlmcUsers []userListItemStruct
var sites []siteListStruct
var groups []groupListStruct

var counters counterTypeStruct
var configFileName string
var configZone string
var configDryRun bool
var configVersion bool
var timeNow string
var startTime time.Time
var endTime time.Duration
var espXmlmc *apiLib.XmlmcInstStruct
var errorCount uint64

//----- Structures -----
type siteListStruct struct {
	SiteName string
	SiteID   int
}
type groupListStruct struct {
	GroupName string
	GroupID   string
}

type xmlmcConfig struct {
	instance string
	zone     string
	url      string
}

type counterTypeStruct struct {
	updated        uint16
	created        uint16
	updatedSkipped uint16
	createskipped  uint16
}
type ldapImportConfStruct struct {
	UserName        string
	Password        string
	InstanceID      string
	UpdateUserType  bool
	URL             string
	LDAPConf        ldapConfStruct
	LDAPMapping     ldapMappingStruct
	LDAPAttirubutes []string
	Roles           []string
	SiteLookup      siteLookupStruct
	OrgLookup       orgLookupStruct
}
type ldapMappingStruct struct {
	UserID         string
	UserType       string
	Name           string
	Password       string
	FirstName      string
	LastName       string
	JobTitle       string
	Site           string
	Phone          string
	Email          string
	Mobile         string
	AbsenceMessage string
	TimeZone       string
	Language       string
	DateTimeFormat string
	DateFormat     string
	TimeFormat     string
	CurrencySymbol string
	CountryCode    string
}
type ldapConfStruct struct {
	Server             string
	UserName           string
	Password           string
	Port               uint16
	ConnectionType     string
	InsecureSkipVerify bool
	Scope              int
	DerefAliases       int
	SizeLimit          int
	TimeLimit          int
	TypesOnly          bool
	Filter             string
	DSN                string
	Debug              bool
}
type siteLookupStruct struct {
	Enabled   bool
	Attribute string
}
type orgLookupStruct struct {
	Enabled     bool
	Attribute   string
	Type        int
	Membership  string
	TasksView   bool
	TasksAction bool
}
type xmlmcResponse struct {
	MethodResult string       `xml:"status,attr"`
	Params       paramsStruct `xml:"params"`
	State        stateStruct  `xml:"state"`
}
type xmlmcCheckUserResponse struct {
	MethodResult string                 `xml:"status,attr"`
	Params       paramsCheckUsersStruct `xml:"params"`
	State        stateStruct            `xml:"state"`
}
type xmlmcUserListResponse struct {
	MethodResult string               `xml:"status,attr"`
	Params       paramsUserListStruct `xml:"params"`
	State        stateStruct          `xml:"state"`
}
type xmlmcuserSetGroupOptionsResponse struct {
	MethodResult string      `xml:"status,attr"`
	State        stateStruct `xml:"state"`
}
type xmlmcSiteListResponse struct {
	MethodResult string               `xml:"status,attr"`
	Params       paramsSiteListStruct `xml:"params"`
	State        stateStruct          `xml:"state"`
}
type paramsSiteListStruct struct {
	RowData paramsSiteRowDataListStruct `xml:"rowData"`
}
type paramsSiteRowDataListStruct struct {
	Row siteObjectStruct `xml:"row"`
}
type siteObjectStruct struct {
	SiteID      int    `xml:"h_id"`
	SiteName    string `xml:"h_site_name"`
	SiteCountry string `xml:"h_country"`
}

type xmlmcGroupListResponse struct {
	MethodResult string                `xml:"status,attr"`
	Params       paramsGroupListStruct `xml:"params"`
	State        stateStruct           `xml:"state"`
}

type paramsGroupListStruct struct {
	RowData paramsGroupRowDataListStruct `xml:"rowData"`
}

type paramsGroupRowDataListStruct struct {
	Row groupObjectStruct `xml:"row"`
}

type groupObjectStruct struct {
	GroupID   string `xml:"h_id"`
	GroupName string `xml:"h_name"`
}
type stateStruct struct {
	Code     string `xml:"code"`
	ErrorRet string `xml:"error"`
}
type paramsCheckUsersStruct struct {
	RecordExist bool `xml:"recordExist"`
}
type paramsStruct struct {
	SessionID string `xml:"sessionId"`
}
type paramsUserListStruct struct {
	UserListItem []userListItemStruct `xml:"userListItem"`
}
type userListItemStruct struct {
	UserID string `xml:"userId"`
	Name   string `xml:"name"`
}

//----- Main Function -----
func main() {
	//-- Start Time for Durration
	startTime = time.Now()
	//-- Start Time for Log File
	timeNow = time.Now().Format(time.RFC3339)
	//-- Remove :
	timeNow = strings.Replace(timeNow, ":", "-", -1)
	//-- Grab Flags
	flag.StringVar(&configFileName, "file", "conf.json", "Name of Configuration File To Load")
	flag.StringVar(&configZone, "zone", "eur", "Override the default Zone the instance sits in")
	flag.BoolVar(&configDryRun, "dryrun", false, "Allow the Import to run without Creating or Updating users")
	flag.BoolVar(&configVersion, "version", false, "Output Version")

	errorCount = 0
	//-- Parse Flags
	flag.Parse()
	if configVersion {
		fmt.Printf("%v \n", version)
		return
	}
	//-- Output
	logger(1, "---- XMLMC LDAP Import Utility V"+fmt.Sprintf("%v", version)+" ----", true)
	logger(1, "Flag - Config File "+fmt.Sprintf("%s", configFileName), true)
	logger(1, "Flag - Zone "+fmt.Sprintf("%s", configZone), true)
	logger(1, "Flag - Dry Run "+fmt.Sprintf("%v", configDryRun), true)
	//--
	//-- Check for latest
	checkVersion()
	//--
	//-- Load Configuration File Into Struct
	ldapImportConf = loadConfig()

	//-- Set Instance ID
	var boolSetInstance = setInstance(configZone, ldapImportConf.InstanceID)
	if boolSetInstance != true {
		return
	}

	//-- Generate Instance XMLMC Endpoint
	ldapImportConf.URL = getInstanceURL()

	//-- Login
	var boolLogin = login()
	if boolLogin != true {
		logger(4, "Unable to Login ", true)
		return
	}
	//-- Query LDAP
	var boolLDAPUsers = queryLdap()

	if boolLDAPUsers {
		processUsers()
	}
	//-- Logout
	logout()

	//-- End output
	if errorCount > 0 {
		logger(4, "Error Count: "+fmt.Sprintf("%d", errorCount), true)
		logger(4, "Check Log File for Details", true)
	}
	logger(1, "Updated: "+fmt.Sprintf("%d", counters.updated), true)
	logger(1, "Updated Skipped: "+fmt.Sprintf("%d", counters.updatedSkipped), true)
	logger(1, "Created: "+fmt.Sprintf("%d", counters.created), true)
	logger(1, "Created Skipped: "+fmt.Sprintf("%d", counters.createskipped), true)

	//-- Show Time Takens
	endTime = time.Now().Sub(startTime)
	logger(1, "Time Taken: "+fmt.Sprintf("%v", endTime), true)
	logger(1, "---- XMLMC LDAP Import Complete ---- ", true)
}

//-- Check Latest
func checkVersion() {
	githubTag := &latest.GithubTag{
		Owner:      "hornbill",
		Repository: "goLDAPUserImport",
	}

	res, err := latest.Check(githubTag, version)
	if err != nil {
		logger(4, fmt.Sprintf("%s", err), true)
		return
	}
	if res.Outdated {
		logger(3, fmt.Sprintf("%s", version)+" is not latest, you should upgrade to "+fmt.Sprintf("%s", res.Current)+" Here https://github.com/hornbill/goLDAPUserImport/releases/tag/v"+fmt.Sprintf("%s", res.Current), true)
	}
}

//-- Function to Load Configruation File
func loadConfig() ldapImportConfStruct {
	//-- Check Config File File Exists
	cwd, _ := os.Getwd()
	configurationFilePath := cwd + "/" + configFileName
	logger(1, "Loading Config File: "+configurationFilePath, false)
	if _, fileCheckErr := os.Stat(configurationFilePath); os.IsNotExist(fileCheckErr) {
		logger(4, "No Configuration File", true)
		os.Exit(102)
	}
	//-- Load Config File
	file, fileError := os.Open(configurationFilePath)
	//-- Check For Error Reading File
	if fileError != nil {
		logger(4, "Error Opening Configuration File: "+fmt.Sprintf("%v", fileError), true)
	}
	//-- New Decoder
	decoder := json.NewDecoder(file)
	//-- New Var based on ldapImportConf
	eldapConf := ldapImportConfStruct{}

	//-- Decode JSON
	err := decoder.Decode(&eldapConf)
	//-- Error Checking
	if err != nil {
		logger(4, "Error Decoding Configuration File: "+fmt.Sprintf("%v", err), true)
	}
	//-- Return New Congfig
	return eldapConf
}

//-- XMLMC Login
func login() bool {
	//-- Check for username and password
	if ldapImportConf.UserName == "" {
		logger(4, "UserName Must be Specified in the Configuration File", true)
		return false
	}
	if ldapImportConf.Password == "" {
		logger(4, "Password Must be Specified in the Configuration File", true)
		return false
	}
	logger(1, "Logging Into: "+ldapImportConf.URL, true)
	logger(1, "UserName: "+ldapImportConf.UserName, true)
	espXmlmc = apiLib.NewXmlmcInstance(ldapImportConf.URL)
	espXmlmc.SetParam("userId", ldapImportConf.UserName)
	espXmlmc.SetParam("password", base64.StdEncoding.EncodeToString([]byte(ldapImportConf.Password)))
	XMLLogin, xmlmcErr := espXmlmc.Invoke("session", "userLogon")
	var xmlRespon xmlmcResponse
	if xmlmcErr != nil {
		logger(4, "Unable to Login - Server Error: "+fmt.Sprintf("%v", xmlmcErr), true)
		return false
	}
	err := xml.Unmarshal([]byte(XMLLogin), &xmlRespon)
	if err != nil {
		logger(4, "Unable to Login: "+fmt.Sprintf("%v", err), true)
		return false
	}
	if xmlRespon.MethodResult != "ok" {
		logger(4, "Unable to Login: "+xmlRespon.State.ErrorRet, true)
		return false
	}
	logger(1, "Successfully Logged into Hornbill", true)
	espLogger("---- XMLMC LDAP User Import Utility V"+fmt.Sprintf("%v", version)+" ----", "debug")
	espLogger("Logged In As: "+ldapImportConf.UserName, "debug")
	return true
}

func connectLDAP() *ldap.LDAPConnection {
	TLSconfig := &tls.Config{
		ServerName:         ldapImportConf.LDAPConf.Server,
		InsecureSkipVerify: ldapImportConf.LDAPConf.InsecureSkipVerify,
	}
	//-- Based on Connection Type Normal | TLS | SSL
	logger(1, "Connecting... \nServer: "+ldapImportConf.LDAPConf.Server+"\nPort: "+fmt.Sprintf("%d", ldapImportConf.LDAPConf.Port)+"\nType: "+ldapImportConf.LDAPConf.ConnectionType+"\nSkip Verify: "+fmt.Sprintf("%t", ldapImportConf.LDAPConf.InsecureSkipVerify)+"\nDebug: "+fmt.Sprintf("%t", ldapImportConf.LDAPConf.Debug), true)
	t := ldapImportConf.LDAPConf.ConnectionType
	switch t {
	case "":
		//-- Normal
		logger(1, "Creating LDAP Connection", false)
		l := ldap.NewLDAPConnection(ldapImportConf.LDAPConf.Server, ldapImportConf.LDAPConf.Port)
		l.Debug = ldapImportConf.LDAPConf.Debug
		return l
	case "TLS":
		//-- TLS
		logger(1, "Creating LDAP Connection (TLS)", false)
		l := ldap.NewLDAPTLSConnection(ldapImportConf.LDAPConf.Server, ldapImportConf.LDAPConf.Port, TLSconfig)
		l.Debug = ldapImportConf.LDAPConf.Debug
		return l
	case "SSL":
		//-- SSL
		logger(1, "Creating LDAP Connection (SSL)", false)
		l := ldap.NewLDAPSSLConnection(ldapImportConf.LDAPConf.Server, ldapImportConf.LDAPConf.Port, TLSconfig)
		l.Debug = ldapImportConf.LDAPConf.Debug
		return l
	}

	return nil
}

//-- Query LDAP
func queryLdap() bool {
	//-- Create LDAP Connection
	l := connectLDAP()
	conErr := l.Connect()
	if conErr != nil {
		logger(4, "Connecting Error: "+fmt.Sprintf("%v", conErr), true)
		return false
	}
	defer l.Close()

	//-- Bind
	bindErr := l.Bind(ldapImportConf.LDAPConf.UserName, ldapImportConf.LDAPConf.Password)
	if bindErr != nil {
		logger(4, "Bind Error: "+fmt.Sprintf("%v", bindErr), true)
		return false
	}
	logger(1, "LDAP Search Query \n"+fmt.Sprintf("%+v", ldapImportConf.LDAPConf)+" ----", false)
	//-- Build Search Request
	searchRequest := ldap.NewSearchRequest(
		ldapImportConf.LDAPConf.DSN,
		ldapImportConf.LDAPConf.Scope,
		ldapImportConf.LDAPConf.DerefAliases,
		ldapImportConf.LDAPConf.SizeLimit,
		ldapImportConf.LDAPConf.TimeLimit,
		ldapImportConf.LDAPConf.TypesOnly,
		ldapImportConf.LDAPConf.Filter,
		ldapImportConf.LDAPAttirubutes,
		nil)

	//-- Search Request with 1000 limit pagaing
	results, searchErr := l.SearchWithPaging(searchRequest, 1000)
	if searchErr != nil {
		logger(4, "Search Error: "+fmt.Sprintf("%v", searchErr), true)
		return false
	}

	logger(1, "LDAP Results: "+fmt.Sprintf("%d", len(results.Entries)), true)
	//-- Catch zero results
	if len(results.Entries) == 0 {
		logger(4, "[LDAP] [SEARCH] No Users Found ", true)
		return false
	}
	ldapUsers = results.Entries
	return true
}

//-- Process Users
func processUsers() {
	bar := pb.StartNew(len(ldapUsers))
	logger(1, "Processing Users", false)
	//-- Loop Each LDAP USER
	for _, ldapUser := range ldapUsers {
		logger(1, "LDAP User Record \n"+fmt.Sprintf("%+v", ldapUser)+" ----", false)
		bar.Increment()
		var boolUpdate = false
		logger(1, "LDAP User: "+getFeildValue(ldapUser, "UserID"), false)
		//-- For Each LDAP Users Check if they already Exist
		var userID = strings.ToLower(getFeildValue(ldapUser, "UserID"))
		boolUpdate = checkUserOnInstance(userID)
		//-- Update or Create User
		if boolUpdate {
			logger(1, "Update User: "+getFeildValue(ldapUser, "UserID"), false)
			updateUser(ldapUser)
		} else {
			logger(1, "Create User: "+getFeildValue(ldapUser, "UserID"), false)
			if ldapUser != nil {
				createUser(ldapUser)
			}

		}
	}
	bar.FinishPrint("Processing Complete!")
}

//-- Does User Exist on Instance
func checkUserOnInstance(userID string) bool {
	//espXmlmc := espXmlmc.NewXmlmcInstance(ldapImportConf.Url)
	espXmlmc.SetParam("entity", "UserAccount")
	espXmlmc.SetParam("keyValue", userID)
	XMLCheckUser, xmlmcErr := espXmlmc.Invoke("data", "entityDoesRecordExist")
	var xmlRespon xmlmcCheckUserResponse
	if xmlmcErr != nil {
		log.Fatal(xmlmcErr)
		return false
	}
	err := xml.Unmarshal([]byte(XMLCheckUser), &xmlRespon)
	if err != nil {
		return false
	}
	if xmlRespon.MethodResult != "ok" {
		logger(4, "Unable to Search User: "+xmlRespon.State.ErrorRet, true)
		return false
	}
	return xmlRespon.Params.RecordExist
}

//-- Function to search for site
func getSiteFromLookup(u *ldap.Entry) string {
	siteReturn := ""
	//-- Check if Site Attribute is set
	if ldapImportConf.SiteLookup.Attribute == "" {
		logger(4, "Site Lookup is Enabled but Attribute is not Defined", true)
		return ""
	}
	//-- Get Value of Attribute
	logger(1, "LDAP Attribute "+ldapImportConf.SiteLookup.Attribute, false)
	siteAttributeName := u.GetAttributeValue(ldapImportConf.SiteLookup.Attribute)
	logger(1, "Looking Up Site "+siteAttributeName, false)
	siteIsInCache, SiteIDCache := siteInCache(siteAttributeName)
	//-- Check if we have Chached the site already
	if siteIsInCache {
		siteReturn = strconv.Itoa(SiteIDCache)
		logger(1, "Found Site in Cache"+siteReturn, false)

	} else {
		siteIsOnInstance, SiteIDInstance := searchSite(siteAttributeName)
		//-- If Returned set output
		if siteIsOnInstance {
			siteReturn = strconv.Itoa(SiteIDInstance)
		}
	}
	logger(1, "Site Lookup found Id "+siteReturn, false)
	return siteReturn
}

//-- Function to Check if in Cache
func siteInCache(siteName string) (bool, int) {
	boolReturn := false
	intReturn := 0
	//-- Check if in Cache
	for _, site := range sites {
		if site.SiteName == siteName {
			boolReturn = true
			intReturn = site.SiteID
		}
	}
	return boolReturn, intReturn
}

//-- Function to Check if site is on the instance
func searchSite(siteName string) (bool, int) {
	boolReturn := false
	intReturn := 0
	//-- ESP Query for site
	//espXmlmc := espXmlmc.NewXmlmcInstance(ldapImportConf.Url)
	if siteName == "" {
		return boolReturn, intReturn
	}
	espXmlmc.SetParam("entity", "Site")
	espXmlmc.SetParam("matchScope", "all")
	espXmlmc.OpenElement("searchFilter")
	espXmlmc.SetParam("h_site_name", siteName)
	espXmlmc.CloseElement("searchFilter")
	espXmlmc.SetParam("maxResults", "1")
	XMLSiteSearch, xmlmcErr := espXmlmc.Invoke("data", "entityBrowseRecords")
	var xmlRespon xmlmcSiteListResponse
	if xmlmcErr != nil {
		log.Fatal(xmlmcErr)
	}
	err := xml.Unmarshal([]byte(XMLSiteSearch), &xmlRespon)
	if err != nil {
		logger(4, "Unable to Search for Site: "+fmt.Sprintf("%v", err), true)
	} else {
		if xmlRespon.MethodResult != "ok" {
			logger(4, "Unable to Search for Site: "+xmlRespon.State.ErrorRet, true)
		} else {
			//-- Check Response
			if xmlRespon.Params.RowData.Row.SiteName != "" {
				if strings.ToLower(xmlRespon.Params.RowData.Row.SiteName) == strings.ToLower(siteName) {
					intReturn = xmlRespon.Params.RowData.Row.SiteID
					boolReturn = true
					//-- Add Site to Cache
					var newSiteForCache siteListStruct
					newSiteForCache.SiteID = intReturn
					newSiteForCache.SiteName = siteName
					name := []siteListStruct{newSiteForCache}
					sites = append(sites, name...)
				}
			}
		}
	}

	return boolReturn, intReturn
}

//-- Update User Record
func updateUser(u *ldap.Entry) bool {
	//-- Do we Lookup Site
	site := ""
	if ldapImportConf.SiteLookup.Enabled {
		site = getSiteFromLookup(u)
	} else {
		site = getFeildValue(u, "Site")
	}

	//espXmlmc := espXmlmc.NewXmlmcInstance(ldapImportConf.Url)
	if getFeildValue(u, "UserID") != "" {
		espXmlmc.SetParam("userId", getFeildValue(u, "UserID"))
	}

	if getFeildValue(u, "UserType") != "" && ldapImportConf.UpdateUserType {
		espXmlmc.SetParam("userType", getFeildValue(u, "UserType"))
	}
	if getFeildValue(u, "Name") != "" {
		espXmlmc.SetParam("name", getFeildValue(u, "Name"))
	}
	if getFeildValue(u, "FirstName") != "" {
		espXmlmc.SetParam("firstName", getFeildValue(u, "FirstName"))
	}
	if getFeildValue(u, "LastName") != "" {
		espXmlmc.SetParam("lastName", getFeildValue(u, "LastName"))
	}
	if getFeildValue(u, "JobTitle") != "" {
		espXmlmc.SetParam("jobTitle", getFeildValue(u, "JobTitle"))
	}
	if site != "" {
		espXmlmc.SetParam("site", site)
	}
	if getFeildValue(u, "Phone") != "" {
		espXmlmc.SetParam("phone", getFeildValue(u, "Phone"))
	}
	if getFeildValue(u, "Email") != "" {
		espXmlmc.SetParam("email", getFeildValue(u, "Email"))
	}
	if getFeildValue(u, "Mobile") != "" {
		espXmlmc.SetParam("mobile", getFeildValue(u, "Mobile"))
	}
	if getFeildValue(u, "AbsenceMessage") != "" {
		espXmlmc.SetParam("absenceMessage", getFeildValue(u, "AbsenceMessage"))
	}
	if getFeildValue(u, "TimeZone") != "" {
		espXmlmc.SetParam("timeZone", getFeildValue(u, "TimeZone"))
	}
	if getFeildValue(u, "Language") != "" {
		espXmlmc.SetParam("language", getFeildValue(u, "Language"))
	}
	if getFeildValue(u, "DateTimeFormat") != "" {
		espXmlmc.SetParam("dateTimeFormat", getFeildValue(u, "DateTimeFormat"))
	}
	if getFeildValue(u, "DateFormat") != "" {
		espXmlmc.SetParam("dateFormat", getFeildValue(u, "DateFormat"))
	}
	if getFeildValue(u, "TimeFormat") != "" {
		espXmlmc.SetParam("timeFormat", getFeildValue(u, "TimeFormat"))
	}
	if getFeildValue(u, "CurrencySymbol") != "" {
		espXmlmc.SetParam("currencySymbol", getFeildValue(u, "CurrencySymbol"))
	}
	if getFeildValue(u, "CountryCode") != "" {
		espXmlmc.SetParam("countryCode", getFeildValue(u, "CountryCode"))
	}
	//-- Check for Dry Run
	if configDryRun != true {
		XMLUpdate, xmlmcErr := espXmlmc.Invoke("admin", "userUpdate")
		var xmlRespon xmlmcResponse
		if xmlmcErr != nil {
			log.Fatal(xmlmcErr)
		}
		err := xml.Unmarshal([]byte(XMLUpdate), &xmlRespon)
		if err != nil {
			return false
		}
		if xmlRespon.MethodResult != "ok" && xmlRespon.State.ErrorRet != "There are no values to update" {
			logger(4, "Unable to Update User: "+xmlRespon.State.ErrorRet, false)
			espLogger("Unable to Update User: "+xmlRespon.State.ErrorRet, "error")
			errorCount++

		} else {

			if ldapImportConf.OrgLookup.Enabled {
				userAddGroup(u)
			}

			if xmlRespon.State.ErrorRet != "There are no values to update" {
				logger(1, "No Changes", false)
				counters.updated++
			} else {
				counters.updatedSkipped++
			}

			return true
		}
	} else {
		//-- Inc Counter
		counters.updatedSkipped++
		//-- DEBUG XML TO LOG FILE
		var XMLSTRING = espXmlmc.GetParam()
		logger(1, "User Update XML "+fmt.Sprintf("%s", XMLSTRING), false)
		espXmlmc.ClearParam()
	}

	return true
}

//-- Create Users
func createUser(u *ldap.Entry) bool {
	//-- Do we Lookup Site
	site := ""
	if ldapImportConf.SiteLookup.Enabled {
		site = getSiteFromLookup(u)
	} else {
		site = getFeildValue(u, "Site")
	}

	//espXmlmc := espXmlmc.NewXmlmcInstance(ldapImportConf.Url)
	if getFeildValue(u, "UserID") != "" {
		espXmlmc.SetParam("userId", getFeildValue(u, "UserID"))
	}
	if getFeildValue(u, "Name") != "" {
		espXmlmc.SetParam("name", getFeildValue(u, "Name"))
	}
	var password = getFeildValue(u, "Password")
	//-- If Password is Blank Generate Password
	if password == "" {
		password = generatePasswordString(10)
		logger(1, "Auto Generated Password for: "+getFeildValue(u, "UserID")+" - "+password, false)
	}
	espXmlmc.SetParam("password", base64.StdEncoding.EncodeToString([]byte(password)))
	if getFeildValue(u, "UserType") != "" {
		espXmlmc.SetParam("userType", getFeildValue(u, "UserType"))
	}
	if getFeildValue(u, "FirstName") != "" {
		espXmlmc.SetParam("firstName", getFeildValue(u, "FirstName"))
	}
	if getFeildValue(u, "LastName") != "" {
		espXmlmc.SetParam("lastName", getFeildValue(u, "LastName"))
	}
	if getFeildValue(u, "JobTitle") != "" {
		espXmlmc.SetParam("jobTitle", getFeildValue(u, "JobTitle"))
	}
	if site != "" {
		espXmlmc.SetParam("site", site)
	}
	if getFeildValue(u, "Phone") != "" {
		espXmlmc.SetParam("phone", getFeildValue(u, "Phone"))
	}
	if getFeildValue(u, "Email") != "" {
		espXmlmc.SetParam("email", getFeildValue(u, "Email"))
	}
	if getFeildValue(u, "Mobile") != "" {
		espXmlmc.SetParam("mobile", getFeildValue(u, "Mobile"))
	}
	if getFeildValue(u, "AbsenceMessage") != "" {
		espXmlmc.SetParam("absenceMessage", getFeildValue(u, "AbsenceMessage"))
	}
	if getFeildValue(u, "TimeZone") != "" {
		espXmlmc.SetParam("timeZone", getFeildValue(u, "TimeZone"))
	}
	if getFeildValue(u, "Language") != "" {
		espXmlmc.SetParam("language", getFeildValue(u, "Language"))
	}
	if getFeildValue(u, "DateTimeFormat") != "" {
		espXmlmc.SetParam("dateTimeFormat", getFeildValue(u, "DateTimeFormat"))
	}
	if getFeildValue(u, "DateFormat") != "" {
		espXmlmc.SetParam("dateFormat", getFeildValue(u, "DateFormat"))
	}
	if getFeildValue(u, "TimeFormat") != "" {
		espXmlmc.SetParam("timeFormat", getFeildValue(u, "TimeFormat"))
	}
	if getFeildValue(u, "CurrencySymbol") != "" {
		espXmlmc.SetParam("currencySymbol", getFeildValue(u, "CurrencySymbol"))
	}
	if getFeildValue(u, "CountryCode") != "" {
		espXmlmc.SetParam("countryCode", getFeildValue(u, "CountryCode"))
	}
	//-- Check for Dry Run
	if configDryRun != true {
		XMLCreate, xmlmcErr := espXmlmc.Invoke("admin", "userCreate")
		var xmlRespon xmlmcResponse
		if xmlmcErr != nil {
			log.Fatal(xmlmcErr)
		}
		err := xml.Unmarshal([]byte(XMLCreate), &xmlRespon)
		if err != nil {
			return false
		}
		if xmlRespon.MethodResult != "ok" {
			logger(4, "Unable to Create User: "+xmlRespon.State.ErrorRet, false)
			espLogger("Unable to Create User: "+xmlRespon.State.ErrorRet, "error")
			errorCount++
		} else {
			if ldapImportConf.OrgLookup.Enabled {
				userAddGroup(u)
			}
			if len(ldapImportConf.Roles) > 0 {
				userAddRoles(getFeildValue(u, "UserID"))
			}
			counters.created++
			return true
		}
	} else {
		//-- DEBUG XML TO LOG FILE
		var XMLSTRING = espXmlmc.GetParam()
		logger(1, "User Create XML "+fmt.Sprintf("%s", XMLSTRING), false)
		counters.createskipped++
		espXmlmc.ClearParam()
	}

	return true
}

//-- Deal with adding a user to a group
func userAddGroup(u *ldap.Entry) bool {

	//-- Check if Site Attribute is set
	if ldapImportConf.OrgLookup.Attribute == "" {
		logger(4, "Org Lookup is Enabled but Attribute is not Defined", true)
		return false
	}
	//-- Get Value of Attribute
	logger(1, "LDAP Attribute "+ldapImportConf.OrgLookup.Attribute, false)
	orgAttributeName := processComplexFeild(u, ldapImportConf.OrgLookup.Attribute)
	logger(1, "Looking Up Org "+orgAttributeName, false)
	orgIsInCache, orgId := groupInCache(orgAttributeName)
	//-- Check if we have Chached the site already
	if orgIsInCache {
		logger(1, "Found Org in Cache "+orgId, false)
		userAddGroupAsoc(u, orgId)
		return true
	} else {
		orgIsOnInstance, orgId := searchOrg(orgAttributeName)

		if orgIsOnInstance {
			logger(1, "Org Lookup found Id "+orgId, false)
			userAddGroupAsoc(u, orgId)
			return true
		}
	}
	logger(1, "Unable to Find Organsiation "+orgAttributeName, false)
	return false
}

func userAddGroupAsoc(u *ldap.Entry, orgId string) {
	userId := getFeildValue(u, "UserID")
	espXmlmc.SetParam("userId", userId)
	espXmlmc.SetParam("groupId", orgId)
	espXmlmc.SetParam("memberRole", ldapImportConf.OrgLookup.Membership)
	espXmlmc.OpenElement("options")
	espXmlmc.SetParam("tasksView", strconv.FormatBool(ldapImportConf.OrgLookup.TasksView))
	espXmlmc.SetParam("tasksAction", strconv.FormatBool(ldapImportConf.OrgLookup.TasksAction))
	espXmlmc.CloseElement("options")

	XMLSiteSearch, xmlmcErr := espXmlmc.Invoke("admin", "userAddGroup")
	var xmlRespon xmlmcuserSetGroupOptionsResponse
	if xmlmcErr != nil {
		log.Fatal(xmlmcErr)
	}
	err := xml.Unmarshal([]byte(XMLSiteSearch), &xmlRespon)
	if err != nil {
		logger(4, "Unable to Associate User To Group: "+fmt.Sprintf("%v", err), true)
	} else {
		if xmlRespon.MethodResult != "ok" {
			if xmlRespon.State.ErrorRet != "The specified user ["+userId+"] already belongs to ["+orgId+"] group" {
				logger(4, "Unable to Associate User To Organsiation: "+xmlRespon.State.ErrorRet, true)
			} else {
				logger(1, "User: "+userId+" Already Added to Organsiation: "+orgId, false)
			}

		} else {
			logger(1, "User: "+userId+" Added to Organsiation: "+orgId, false)
		}
	}

}

//-- Function to Check if in Cache
func groupInCache(groupName string) (bool, string) {
	boolReturn := false
	stringReturn := ""
	//-- Check if in Cache
	for _, group := range groups {
		if group.GroupName == groupName {
			boolReturn = true
			stringReturn = group.GroupID
		}
	}

	return boolReturn, stringReturn
}

//-- Function to Check if site is on the instance
func searchOrg(orgName string) (bool, string) {
	boolReturn := false
	strReturn := ""
	//-- ESP Query for site
	//espXmlmc := espXmlmc.NewXmlmcInstance(ldapImportConf.Url)
	if orgName == "" {
		return boolReturn, strReturn
	}
	espXmlmc.SetParam("application", "com.hornbill.core")
	espXmlmc.SetParam("queryName", "GetGroupByName")
	espXmlmc.OpenElement("queryParams")
	espXmlmc.SetParam("h_name", orgName)
	espXmlmc.SetParam("h_type", strconv.Itoa(ldapImportConf.OrgLookup.Type))
	espXmlmc.CloseElement("queryParams")

	XMLSiteSearch, xmlmcErr := espXmlmc.Invoke("data", "queryExec")
	var xmlRespon xmlmcGroupListResponse
	if xmlmcErr != nil {
		log.Fatal(xmlmcErr)
	}
	err := xml.Unmarshal([]byte(XMLSiteSearch), &xmlRespon)
	if err != nil {
		logger(4, "Unable to Search for Group: "+fmt.Sprintf("%v", err), true)
	} else {
		if xmlRespon.MethodResult != "ok" {
			logger(4, "Unable to Search for Group: "+xmlRespon.State.ErrorRet, true)
		} else {
			//-- Check Response
			if xmlRespon.Params.RowData.Row.GroupID != "" {
				strReturn = xmlRespon.Params.RowData.Row.GroupID
				boolReturn = true
				//-- Add Group to Cache
				var newgroupForCache groupListStruct
				newgroupForCache.GroupID = strReturn
				newgroupForCache.GroupName = orgName
				name := []groupListStruct{newgroupForCache}
				groups = append(groups, name...)

			}
		}
	}

	return boolReturn, strReturn
}

func userAddRoles(userID string) bool {
	//espXmlmc := espXmlmc.NewXmlmcInstance(ldapImportConf.Url)
	espXmlmc.SetParam("userId", userID)
	for _, role := range ldapImportConf.Roles {
		espXmlmc.SetParam("role", role)
		logger(1, "Add Role to User: "+role, false)
	}
	XMLCreate, xmlmcErr := espXmlmc.Invoke("admin", "userAddRole")
	var xmlRespon xmlmcResponse
	if xmlmcErr != nil {
		log.Fatal(xmlmcErr)
	}
	err := xml.Unmarshal([]byte(XMLCreate), &xmlRespon)
	if err != nil {
		return false
	}
	if xmlRespon.MethodResult != "ok" {
		logger(4, "Unable to Assign Role to User: "+xmlRespon.State.ErrorRet, true)
		espLogger("Unable to Assign Role to User: "+xmlRespon.State.ErrorRet, "error")
		return false
	}
	logger(1, "Roles Added Successfully", false)
	return true
}

//-- Get XMLMC Feild from mapping via LDAP Object
func getFeildValue(u *ldap.Entry, s string) string {
	//-- Dyniamicly Grab Mapped Value
	r := reflect.ValueOf(ldapImportConf.LDAPMapping)
	f := reflect.Indirect(r).FieldByName(s)
	//-- Get Mapped Value
	var LDAPMapping = f.String()
	return processComplexFeild(u, LDAPMapping)
}
func processComplexFeild(u *ldap.Entry, s string) string {
	//-- Match $variables from String
	re1, err := regexp.Compile(`\[(.*?)\]`)
	if err != nil {
		fmt.Printf("[ERROR] %v", err)
	}
	//-- Get Array of all Matched max 100
	result := re1.FindAllString(s, 100)

	//-- Loop Matches
	for _, v := range result {
		//-- Grab LDAP Mapping value from result set
		var LDAPAttributeValue = u.GetAttributeValue(v[1 : len(v)-1])
		//-- Check for Invalid Value
		if LDAPAttributeValue == "" {
			logger(4, "Unable to Load LDAP Attribute: "+v[1:len(v)-1]+" For Input Param: "+s, false)
			return LDAPAttributeValue
		}
		s = strings.Replace(s, v, LDAPAttributeValue, 1)
	}

	//-- Return Value
	return s
}

//-- Generate Password String
func generatePasswordString(n int) string {
	var arbytes = make([]byte, n)
	rand.Read(arbytes)
	for i, b := range arbytes {
		arbytes[i] = letterBytes[b%byte(len(letterBytes))]
	}
	return string(arbytes)
}

//-- Loggin function
func logger(t int, s string, outputtoCLI bool) {
	//-- Curreny WD
	cwd, _ := os.Getwd()
	//-- Log Folder
	logPath := cwd + "/log"
	//-- Log File
	logFileName := logPath + "/LDAP_User_Import_" + timeNow + ".log"
	red := color.New(color.FgRed).PrintfFunc()
	orange := color.New(color.FgCyan).PrintfFunc()
	//-- If Folder Does Not Exist then create it
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		err := os.Mkdir(logPath, 0777)
		if err != nil {
			fmt.Printf("Error Creating Log Folder %q: %s \r", logPath, err)
			os.Exit(101)
		}
	}

	//-- Open Log File
	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		fmt.Printf("Error Creating Log File %q: %s \n", logFileName, err)
		os.Exit(100)
	}
	// don't forget to close it
	defer f.Close()
	// assign it to the standard logger
	log.SetOutput(f)
	var errorLogPrefix = ""
	//-- Create Log Entry
	switch t {
	case 1:
		errorLogPrefix = "[DEBUG] "
	case 2:
		errorLogPrefix = "[MESSAGE] "
	case 3:
		errorLogPrefix = "[WARN] "
	case 4:
		errorLogPrefix = "[ERROR] "
	}
	if outputtoCLI {
		if t == 3 {
			orange(errorLogPrefix + s + "\n")
		} else if t == 4 {
			red(errorLogPrefix + s + "\n")
		} else {
			fmt.Printf(errorLogPrefix + s + "\n")
		}

	}
	log.Println(errorLogPrefix + s)
}

//-- XMLMC LogOut
func logout() {
	//-- End output
	espLogger("Errors: "+fmt.Sprintf("%d", errorCount), "error")
	espLogger("Updated: "+fmt.Sprintf("%d", counters.updated), "debug")
	espLogger("Updated Skipped: "+fmt.Sprintf("%d", counters.updatedSkipped), "debug")
	espLogger("Created: "+fmt.Sprintf("%d", counters.created), "debug")
	espLogger("Created Skipped: "+fmt.Sprintf("%d", counters.createskipped), "debug")
	espLogger("Time Taken: "+fmt.Sprintf("%v", endTime), "debug")
	espLogger("---- XMLMC LDAP User Import Complete ---- ", "debug")
	logger(1, "Logout", true)
	//espXmlmc := espXmlmc.NewXmlmcInstance(ldapImportConf.Url)
	espXmlmc.Invoke("session", "userLogoff")
}

// Set Instance Id
func setInstance(strZone string, instanceID string) bool {
	//-- Set Zone
	setZone(strZone)
	//-- Check for blank instance
	if instanceID == "" {
		logger(4, "InstanceId Must be Specified in the Configuration File", true)
		return false
	}
	//-- Set Instance
	xmlmcInstanceConfig.instance = instanceID
	return true
}

// Set Instance Zone to Overide Live
func setZone(zone string) {
	xmlmcInstanceConfig.zone = zone

	return
}

//-- Log to ESP
func espLogger(message string, severity string) {
	//espXmlmc := espXmlmc.NewXmlmcInstance(ldapImportConf.Url)
	espXmlmc.SetParam("fileName", "LDAP_User_Import")
	espXmlmc.SetParam("group", "general")
	espXmlmc.SetParam("severity", severity)
	espXmlmc.SetParam("message", message)
	espXmlmc.Invoke("system", "logMessage")
}

//-- Function Builds XMLMC End Point
func getInstanceURL() string {
	xmlmcInstanceConfig.url = "https://"
	xmlmcInstanceConfig.url += xmlmcInstanceConfig.zone
	xmlmcInstanceConfig.url += "api.hornbill.com/"
	xmlmcInstanceConfig.url += xmlmcInstanceConfig.instance
	xmlmcInstanceConfig.url += "/xmlmc/"

	return xmlmcInstanceConfig.url
}
