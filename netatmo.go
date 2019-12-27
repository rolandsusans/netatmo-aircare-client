package netatmo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"golang.org/x/oauth2"
)

const (
	// DefaultBaseURL is netatmo api url
	baseURL = "https://api.netatmo.net/"
	// DefaultAuthURL is netatmo auth url
	authURL = baseURL + "oauth2/token"
	// DefaultDeviceURL is netatmo device url
	deviceURL = baseURL + "/api/gethomecoachsdata"
)

// Config is used to specify credential to Netatmo API
// ClientID : Client ID from netatmo app registration at http://dev.netatmo.com/dev/listapps
// ClientSecret : Client app secret
// Username : Your netatmo account username
// Password : Your netatmo account password
type Config struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

// Client use to make request to Netatmo API
type Client struct {
	oauth        *oauth2.Config
	httpClient   *http.Client
	httpResponse *http.Response
	Dc           *DeviceCollection
}

// DeviceCollection hold all devices from netatmo account
type DeviceCollection struct {
	Body struct {
		Devices []*Device `json:"devices"`
	}
}

// Device is a station or a module
// ID : Mac address
// StationName : Station name (only for station)
// ModuleName : Module name
// BatteryPercent : Percentage of battery remaining
// WifiStatus : Wifi status per Base station
// RFStatus : Current radio status per module
// Type : Module type :
//  "NAMain" : for the base station
//  "NAModule1" : for the outdoor module
//  "NAModule4" : for the additionnal indoor module
//  "NAModule3" : for the rain gauge module
//  "NAModule2" : for the wind gauge module
// DashboardData : Data collection from device sensors
// DataType : List of available datas
// LinkedModules : Associated modules (only for station)
type Device struct {
	ID             string `json:"_id"`
	StationName    string `json:"station_name"`
	ModuleName     string `json:"module_name"`
	BatteryPercent *int32 `json:"battery_percent,omitempty"`
	WifiStatus     *int32 `json:"wifi_status,omitempty"`
	RFStatus       *int32 `json:"rf_status,omitempty"`
	Type           string
	DashboardData  DashboardData `json:"dashboard_data"`
	LinkedModules []*Device `json:"modules"`
}

// DashboardData is used to store sensor values
// Temperature : Last temperature measure @ LastMeasure (in °C)
// Humidity : Last humidity measured @ LastMeasure (in %)
// CO2 : Last Co2 measured @ time_utc (in ppm)
// Noise : Last noise measured @ LastMeasure (in db)
// Pressure : Last Sea level pressure measured @ LastMeasure (in mb)
// LastMeasure : Contains timestamp of last data received
type DashboardData struct {
	Temperature      *float32 `json:"Temperature,omitempty"` // use pointer to detect ommitted field by json mapping
	Humidity         *int32   `json:"Humidity,omitempty"`
	CO2              *int32   `json:"CO2,omitempty"`
	Noise            *int32   `json:"Noise,omitempty"`
	Pressure         *float32 `json:"Pressure,omitempty"`
	AbsolutePressure *float32 `json:"AbsolutePressure,omitempty"`
	LastMeasure      *int64   `json:"time_utc"`
	HealthIdx        *int32   `json:"health_idx,omitempty"`
}

// NewClient create a handle authentication to Netamo API
func NewClient(config Config) (*Client, error) {
	oauth := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Scopes:       []string{"read_homecoach"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  baseURL,
			TokenURL: authURL,
		},
	}

	token, err := oauth.PasswordCredentialsToken(oauth2.NoContext, config.Username, config.Password)

	return &Client{
		oauth:      oauth,
		httpClient: oauth.Client(oauth2.NoContext, token),
		Dc:         &DeviceCollection{},
	}, err
}

// do a url encoded HTTP POST request
func (c *Client) doHTTPPostForm(url string, data url.Values) (*http.Response, error) {

	req, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	//req.ContentLength = int64(reader.Len())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.doHTTP(req)
}

// send http GET request
func (c *Client) doHTTPGet(url string, data url.Values) (*http.Response, error) {
	if data != nil {
		url = url + "?" + data.Encode()
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return c.doHTTP(req)
}

// do a generic HTTP request
func (c *Client) doHTTP(req *http.Request) (*http.Response, error) {

	// debug
	//debug, _ := httputil.DumpRequestOut(req, true)
	//fmt.Printf("%s\n\n", debug)

	var err error
	c.httpResponse, err = c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return c.httpResponse, nil
}

// process HTTP response
// Unmarshall received data into holder struct
func processHTTPResponse(resp *http.Response, err error, holder interface{}) error {
	defer resp.Body.Close()
	if err != nil {
		return err
	}

	// debug
	//debug, _ := httputil.DumpResponse(resp, true)
	//fmt.Printf("%s\n\n", debug)

	// check http return code
	if resp.StatusCode != 200 {
		//bytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Bad HTTP return code %d", resp.StatusCode)
	}

	// Unmarshall response into given struct
	if err = json.NewDecoder(resp.Body).Decode(holder); err != nil {
		return err
	}

	return nil
}

// GetStations returns the list of stations owned by the user, and their modules
func (c *Client) Read() (*DeviceCollection, error) {
	resp, err := c.doHTTPGet(deviceURL, url.Values{})
	//dc := &DeviceCollection{}

	if err = processHTTPResponse(resp, err, c.Dc); err != nil {
		return nil, err
	}

	return c.Dc, nil
}

// Devices returns the list of devices
func (dc *DeviceCollection) Devices() []*Device {
	return dc.Body.Devices
}

// Stations is an alias of Devices
func (dc *DeviceCollection) Stations() []*Device {
	return dc.Devices()
}

// Modules returns associated device module
func (d *Device) Modules() []*Device {
	modules := d.LinkedModules
	modules = append(modules, d)

	return modules
}

// Data returns timestamp and the list of sensor value for this module
func (d *Device) Data() (int64, map[string]interface{}) {

	// return only populate field of DashboardData
	m := make(map[string]interface{})

	if d.DashboardData.Temperature != nil {
		m["Temperature"] = *d.DashboardData.Temperature
	}
	if d.DashboardData.Humidity != nil {
		m["Humidity"] = *d.DashboardData.Humidity
	}
	if d.DashboardData.CO2 != nil {
		m["CO2"] = *d.DashboardData.CO2
	}
	if d.DashboardData.Noise != nil {
		m["Noise"] = *d.DashboardData.Noise
	}
	if d.DashboardData.Pressure != nil {
		m["Pressure"] = *d.DashboardData.Pressure
	}
	if d.DashboardData.AbsolutePressure != nil {
		m["AbsolutePressure"] = *d.DashboardData.AbsolutePressure
        }
        if d.DashboardData.LastMeasure != nil {
		m["LastMeasure"] = *d.DashboardData.LastMeasure
        }

        if d.DashboardData.HealthIdx != nil {
		m["HealthIdx"] = *d.DashboardData.HealthIdx
        }

	return *d.DashboardData.LastMeasure, m
}

// Info returns timestamp and the list of info value for this module
func (d *Device) Info() (int64, map[string]interface{}) {

	// return only populate field of DashboardData
	m := make(map[string]interface{})

	// Return data from module level
	if d.BatteryPercent != nil {
		m["BatteryPercent"] = *d.BatteryPercent
	}
	if d.WifiStatus != nil {
		m["WifiStatus"] = *d.WifiStatus
	}
	if d.RFStatus != nil {
		m["RFStatus"] = *d.RFStatus
	}

	return *d.DashboardData.LastMeasure, m
}
